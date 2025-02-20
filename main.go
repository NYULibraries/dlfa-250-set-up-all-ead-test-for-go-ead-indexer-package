package main

import (
	"errors"
	"fmt"
	"github.com/nyulibraries/go-ead-indexer/pkg/ead"
	"github.com/nyulibraries/go-ead-indexer/pkg/ead/collectiondoc"
	"github.com/nyulibraries/go-ead-indexer/pkg/ead/component"
	"github.com/nyulibraries/go-ead-indexer/pkg/ead/eadutil"
	"github.com/nyulibraries/go-ead-indexer/pkg/util"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"time"
)

const actualFileSuffix = "-add.xml"
const diffFileSuffix = "-add.txt"
const goldenFileSuffix = "-add.txt"

var diffsDirPath string
var eadDirPath string
var goldenFilesDirPath string
var rootPath string
var tmpFilesDirPath string

var httpHeadersRegExp = regexp.MustCompile("(?ms)^POST.*\r\n\r\n")

// For https://jira.nyu.edu/browse/DLFA-243
// Can't use this:
// &lt;em&gt;(?!.*&lt;em&gt;)(.*?)&amp;lt;/unittitle&amp;gt;&lt;/em&gt;
// ...because Go does not support negative lookahead.  We instead allow this
// regexp to capture the largest match which would include the nested sub-match
// we need to actually work with, and let a separate non-regexp-based process
// take care of the rest.
var emUnittitleMassage = regexp.MustCompile(`&lt;em&gt;(.*?)&amp;lt;/unittitle&amp;gt;&lt;/em&gt;`)

// Go \s metachar is [\t\n\f\r ], and does not include NBSP.
// Source: https://pkg.go.dev/regexp/syntax
var multipleConsecutiveWhitespace = regexp.MustCompile(`[\s ]{2}\s*`)
var leadingWhitespaceInFieldContent = regexp.MustCompile(`>[\s ]+`)
var trailingWhitespaceInFieldContent = regexp.MustCompile(`[\s ]+</field>`)

// We need to get the absolute path to this package in order to get the absolute
// path to the tmp/ directory.  We don't want the wrong directories clobbered by
// the output if this script is run from somewhere outside of this directory.
func init() {
	// The `filename` string is the absolute path to this source file, which should
	// be located at the root of the package directory.
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		log.Panic("ERROR: `runtime.Caller(0)` failed")
	}

	rootPath = filepath.Dir(filename)
}

func abortBadUsage(err error) {
	if err != nil {
		log.Println(err.Error())
	}
	usage()
	os.Exit(1)
}

func clean() error {
	err := os.RemoveAll(diffsDirPath)
	if err != nil {
		return err
	}
	err = os.MkdirAll(diffsDirPath, 0700)
	if err != nil {
		return err
	}

	err = os.RemoveAll(tmpFilesDirPath)
	if err != nil {
		return err
	}
	err = os.MkdirAll(tmpFilesDirPath, 0700)
	if err != nil {
		return err
	}

	_, err = os.Create(filepath.Join(tmpFilesDirPath, ".gitkeep"))
	if err != nil {
		return err
	}

	return nil
}

func getEADValue(testEAD string) (string, error) {
	return getTestdataFileContents(getEADFilePath(testEAD))
}

func getEADFilePath(testEAD string) string {
	return filepath.Join(eadDirPath, testEAD+".xml")
}
func getGoldenFileIDs(eadID string) []string {
	goldenFileIDs := []string{}

	err := filepath.WalkDir(filepath.Join(goldenFilesDirPath, eadID),
		func(path string, dirEntry fs.DirEntry, err error) error {
			if !dirEntry.IsDir() &&
				filepath.Ext(path) == goldenFileSuffix &&
				!strings.HasSuffix(path, "-commit-add") {

				goldenFileIDs = append(goldenFileIDs, strings.TrimSuffix(filepath.Base(path),
					"-add.txt"))
			}
			return nil
		})
	if err != nil {
		log.Panic(fmt.Sprintf(`getGoldenFileIDs("%s") failed: %s`, eadID, err))
	}

	// The slice might already be sorted, but just in case, sort it.
	// Sorting helps with gauging progress by tailing the logs, and helps with
	// debugging in case log messages don't clearly indicate where an error
	// occurred -- in such cases the last successfully tested EAD file lets us
	// know where to start looking.
	slices.Sort(goldenFileIDs)

	return goldenFileIDs
}

func getGoldenFilePath(testEAD string, fileID string) string {
	return filepath.Join(goldenFilesDirPath, testEAD, fileID+"-add.txt")
}

func getGoldenFileValue(eadID string, fileID string) (string, error) {
	fileContents, err := getTestdataFileContents(getGoldenFilePath(eadID, fileID))
	if err != nil {
		return "", err
	}

	xmlBody := httpHeadersRegExp.ReplaceAllString(fileContents, "")

	return xmlBody, nil
}

func getTestdataFileContents(filename string) (string, error) {
	bytes, err := os.ReadFile(filename)

	if err != nil {
		return filename, err
	}

	return string(bytes), nil
}

func getTestEADs() []string {
	testEADs := []string{}

	err := filepath.WalkDir(eadDirPath, func(path string, dirEntry fs.DirEntry, err error) error {
		if !dirEntry.IsDir() && filepath.Ext(path) == ".xml" {
			repositoryCode := filepath.Base(filepath.Dir(path))
			eadID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
			testEADs = append(testEADs, fmt.Sprintf("%s/%s", repositoryCode, eadID))
		}
		return nil
	})
	if err != nil {
		log.Panic(fmt.Sprintf(`getTestEADs() failed: %s`, err))

	}

	return testEADs
}

func isDirectory(path string) bool {
	fileInfo, err := os.Stat(path)
	if err != nil {
		log.Panic(fmt.Sprintf(`isDirectory("%s") failed with error: %s`, path, err))
	}

	return fileInfo.IsDir()
}

// https://jira.nyu.edu/browse/DLFA-243
// Applied to all golden files without exception.
func massageGoldenAll(golden string) string {
	// DLFA-243: "Convert all "&nbsp;" strings in golden files to actual NBSP characters."
	massagedGolden := strings.ReplaceAll(golden, "&amp;nbsp;", " ")

	// DLFA-243: "Remove erroneously inserted EAD tags in Solr field content from golden files."
	// This is the second part of the massage.  The first part is dealt with in
	// `massageGoldenFileIDSpecific()`.
	// Example of what's being fixed here:
	// This:
	//     <unittitle><title render="italic">Ayuda Medica Internacional</title>(photocopied clippings and notes) <title render="italic"></title></unittitle>
	// ...is mangled by v1 indexer into:
	//     <field name="unittitle_ssm">&lt;em&gt;Ayuda Medica Internacional&lt;/em&gt;(photocopied clippings and notes) &lt;em&gt;&amp;lt;/unittitle&amp;gt;&lt;/em&gt;</field>
	//
	// This first set of matches might include the nested sub-match we actually
	// care about.  Go does not support negative lookahead so we settle for this
	// wide net casting and then use non-regexp-based processing to take care of
	// the rest.
	matches := emUnittitleMassage.FindStringSubmatch(massagedGolden)
	if len(matches) == 2 {
		// Isolate the rightmost match.
		lastOccurrenceIndex := strings.LastIndex(matches[0], "&lt;em&gt")
		lastOccurrence := matches[0][lastOccurrenceIndex:]
		// Set up the replacement based on the rightmost match.
		matches = emUnittitleMassage.FindStringSubmatch(lastOccurrence)
		cleanString := "&lt;em&gt;&lt;/em&gt;" + matches[1]
		// Do the replacement everywhere.
		massagedGolden = strings.ReplaceAll(massagedGolden, lastOccurrence, cleanString)
	} else {
		// Do nothing.
	}

	// Whitespace massages
	massagedGolden = strings.ReplaceAll(massagedGolden, "\n", " ")
	massagedGolden = multipleConsecutiveWhitespace.ReplaceAllString(
		massagedGolden, " ")
	massagedGolden = strings.ReplaceAll(massagedGolden,
		"&lt;/em&gt; &lt;em&gt;", "&lt;/em&gt;&lt;em&gt;")
	massagedGolden = leadingWhitespaceInFieldContent.ReplaceAllString(
		massagedGolden, ">")
	massagedGolden = trailingWhitespaceInFieldContent.ReplaceAllString(
		massagedGolden, `</field>`)

	return massagedGolden
}

// https://jira.nyu.edu/browse/DLFA-243
func massageGoldenFileIDSpecific(golden string, fileID string) string {
	var massagedGolden = golden

	// These changes couldn't be handled by the code which deals with the
	// general case for this v1 indexer bug:
	// https://jira.nyu.edu/browse/DLFA-211?focusedCommentId=11487878&page=com.atlassian.jira.plugin.system.issuetabpanels:comment-tabpanel#comment-11487878
	// Since it's only a couple of files, we brute force them.
	if fileID == "alba_218aspace_ref45" {
		massagedGolden = strings.ReplaceAll(massagedGolden,
			"&lt;em&gt;(some with &amp;lt;title render=\"italic\"/&amp;gt;annotations by Friedman)&amp;lt;/unittitle&amp;gt;&lt;/em&gt;",
			"&lt;em&gt;&lt;/em&gt;(some with &lt;em&gt;&lt;/em&gt;annotations by Friedman)",
		)
	} else if fileID == "alba_236aspace_ref26" {
		massagedGolden = strings.ReplaceAll(massagedGolden,
			"&lt;em&gt;George Seldes, &amp;lt;title render=\"italic\"&amp;gt;\"&lt;/em&gt;",
			"&lt;em&gt;&lt;/em&gt;George Seldes, &lt;em&gt;\"&lt;/em&gt;",
		)
	}

	return massagedGolden
}

func parseEADID(testEAD string) string {
	return filepath.Base(testEAD)
}

func parseRepositoryCode(testEAD string) string {
	return filepath.Dir(testEAD)
}

func setDirectoryPaths() {
	args := os.Args

	if len(args) < 3 {
		abortBadUsage(fmt.Errorf("Wrong number of args"))
	}

	// Declare `err` instead of doing `eadDirPath, err :=`, which shadows package
	// level var `eadDirPath`.
	var err error
	eadDirPath, err = filepath.Abs(args[1])
	// Very basic validation of directories:
	// - No error when resolving to absolute path
	// - Is a directory and not a symlink -- because `filepath.WalkDir` does not
	//   work on symlinks.
	// - Directory name matches repo name.  This is not strictly necessary, because
	//   we should be able to rename the directory if we want, but this is just
	//   a one-off test (for now -- maybe we'll make a permanent one later).
	//   This also has the benefit of preventing possible accidental use of the
	//   FABified repo, which has a different name.
	if err != nil || !isDirectory(eadDirPath) || !strings.HasSuffix(eadDirPath, "findingaids_eads_v2") {
		abortBadUsage(fmt.Errorf(`Path "%s" is not a valid findingaids_eads_v2 repo path`, args[1]))
	}

	goldenFilesDirPath, err = filepath.Abs(args[2])
	if err != nil || !isDirectory(goldenFilesDirPath) || !strings.HasSuffix(goldenFilesDirPath, "http-requests") {
		abortBadUsage(fmt.Errorf(`Path "%s" is not a valid dlfa-188_v1-indexer-http-requests repo http-requests/ subdirectory`, args[2]))
	}

	diffsDirPath = filepath.Join(rootPath, "diffs")
	tmpFilesDirPath = filepath.Join(rootPath, "tmp", "actual")
}

func testCollectionDocSolrAddMessage(testEAD string,
	solrAddMessage collectiondoc.SolrAddMessage) error {
	eadID := parseEADID(testEAD)

	return testSolrAddMessageXML(testEAD, eadID, fmt.Sprintf("%s", solrAddMessage))
}

func testComponentSolrAddMessage(testEAD string, fileID string,
	solrAddMessage component.SolrAddMessage) error {

	return testSolrAddMessageXML(testEAD, fileID, fmt.Sprintf("%s", solrAddMessage))
}

func testNoMissingComponents(testEAD string, componentIDs []string) error {
	missingComponents := []string{}

	goldenFileIDs := getGoldenFileIDs(testEAD)
	goldenFileIDs = slices.DeleteFunc(goldenFileIDs, func(goldenFileID string) bool {
		return goldenFileID == parseEADID(testEAD)
	})

	for _, goldenFileID := range goldenFileIDs {
		if !slices.Contains(componentIDs, goldenFileID) {
			missingComponents = append(missingComponents, goldenFileID)
		}
	}

	if len(missingComponents) > 0 {
		slices.SortStableFunc(missingComponents, func(a string, b string) int {
			return strings.Compare(a, b)
		})
		failMessage := fmt.Sprintf("`EAD.Components` for testEAD %s is missing the following component IDs:\n%s",
			testEAD, strings.Join(missingComponents, "\n"))
		return fmt.Errorf(failMessage)
	}

	return nil
}

func testSolrAddMessageXML(testEAD string, fileID string,
	actualValue string) error {

	goldenValue, err := getGoldenFileValue(testEAD, fileID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// This is a test fail, not a fatal test execution error.
			// A missing golden file means that a Solr add message was created
			// for a component that shouldn't exist.
			return fmt.Errorf("No golden file exists for \"%s\": %s",
				fileID, err)
		} else {
			return fmt.Errorf("Error retrieving golden value for \"%s\": %s",
				fileID, err)
		}
	}

	// https://jira.nyu.edu/browse/DLFA-243
	massageGoldenValueStep1 := massageGoldenFileIDSpecific(goldenValue, fileID)
	massagedGoldenValue := massageGoldenAll(massageGoldenValueStep1)

	if actualValue != massagedGoldenValue {
		err := writeActualSolrXMLToTmp(testEAD, fileID, actualValue)
		if err != nil {
			return fmt.Errorf("Error writing actual temp file for test case \"%s/%s\": %s",
				testEAD, fileID, err)
		}

		prettifiedMassagedGolden := eadutil.PrettifySolrAddMessageXML(massagedGoldenValue)
		prettifiedActual := eadutil.PrettifySolrAddMessageXML(actualValue)

		diff := util.DiffStrings("golden [PRETTIFIED]", prettifiedMassagedGolden,
			"actual [PRETTIFIED]", prettifiedActual)
		err = writeDiffFile(testEAD, fileID, diff)
		if err != nil {
			return fmt.Errorf("Error writing diff file for test case \"%s/%s\": %s",
				testEAD, fileID, err)
		}

		return fmt.Errorf("%s golden and actual values do not match\n", fileID)
	}

	return nil
}

func diffFile(testEAD string, fileID string) string {
	return filepath.Join(diffsDirPath, testEAD, fileID+diffFileSuffix)
}

func tmpFile(testEAD string, fileID string) string {
	return filepath.Join(tmpFilesDirPath, testEAD, fileID+actualFileSuffix)
}

func usage() {
	log.Println("usage: go run main.go [path to findingaids_eads_v2] [path to dlfa-188_v1-indexer-http-requests-xml/http-requests/]")
}

func writeActualSolrXMLToTmp(testEAD string, fileID string, actual string) error {
	tmpFile := tmpFile(testEAD, fileID)
	err := os.MkdirAll(filepath.Dir(tmpFile), 0755)
	if err != nil {
		return err
	}

	return os.WriteFile(tmpFile, []byte(actual), 0644)
}

func writeDiffFile(testEAD string, fileID string, diff string) error {
	diffFile := diffFile(testEAD, fileID)
	err := os.MkdirAll(filepath.Dir(diffFile), 0755)
	if err != nil {
		return err
	}

	return os.WriteFile(diffFile, []byte(diff), 0644)
}

func main() {
	setDirectoryPaths()

	err := clean()
	if err != nil {
		log.Panic("clean() error: " + err.Error())
	}

	testEADs := getTestEADs()

	for _, testEAD := range testEADs {
		fmt.Printf("[ %s ] Testing %s\n", time.Now().Format("2006-01-02 15:04:05"), testEAD)
		eadXML, err := getEADValue(testEAD)
		if err != nil {
			log.Println(fmt.Sprintf(`getEADValue("%s") failed: %s`, testEAD, err))
		}

		repositoryCode := parseRepositoryCode(testEAD)
		eadToTest, err := ead.New(repositoryCode, eadXML)
		if err != nil {
			log.Println(fmt.Sprintf(`ead.New("%s", [EADXML for %s ]) failed: %s`, repositoryCode, testEAD, err))
		}

		err = testCollectionDocSolrAddMessage(testEAD, eadToTest.CollectionDoc.SolrAddMessage)
		if err != nil {
			log.Println(err.Error())
		}

		if eadToTest.Components == nil {
			fmt.Println(testEAD + " has no components.  Skipping component tests")

			continue
		}

		componentIDs := []string{}
		for _, component := range *eadToTest.Components {
			componentIDs = append(componentIDs, component.ID)
			err = testComponentSolrAddMessage(testEAD, component.ID,
				component.SolrAddMessage)
			if err != nil {
				log.Println(err.Error())
			}
		}

		err = testNoMissingComponents(testEAD, componentIDs)
		if err != nil {
			log.Println(err.Error())
		}
	}
}
