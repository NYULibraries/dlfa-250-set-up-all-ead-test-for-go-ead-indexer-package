package main

import (
	"errors"
	"fmt"
	"go-ead-indexer/pkg/ead"
	"go-ead-indexer/pkg/ead/collectiondoc"
	"go-ead-indexer/pkg/ead/component"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

const goldenFileSuffix = "-add.xml"

var eadDirPath string
var goldenFilesDirPath string
var rootPath string
var tmpFilesDirPath string

// We need to get the absolute path to this package in order to get the absolute
// path to the tmp/ directory.  We don't want the wrong directories clobbered by
// the output if this script is run from somewhere outside of this directory.
func init() {
	// The `filename` string is the absolute path to this source file, which should
	// be located at the root of the package directory.
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("ERROR: `runtime.Caller(0)` failed")
	}

	rootPath = filepath.Dir(filename)
}

func abortBadUsage(err error) {
	if err != nil {
		println(err.Error())
	}
	usage()
	os.Exit(1)
}

func clean() error {
	err := os.RemoveAll(tmpFilesDirPath)
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
					"-add.xml"))
			}
			return nil
		})
	if err != nil {
		panic(fmt.Sprintf(`getGoldenFileIDs("%s") failed: %s`, eadID, err))
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
	return filepath.Join(goldenFilesDirPath, testEAD, fileID+"-add.xml")
}

func getGoldenFileValue(eadID string, fileID string) (string, error) {
	return getTestdataFileContents(getGoldenFilePath(eadID, fileID))
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
		panic(fmt.Sprintf(`getTestEADs() failed: %s`, err))

	}

	return testEADs
}

func isDirectory(path string) bool {
	fileInfo, err := os.Stat(path)
	if err != nil {
		panic(fmt.Sprintf(`isDirectory("%s") failed with error: %s`, path, err))
	}

	return fileInfo.IsDir()
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
		abortBadUsage(fmt.Errorf(`Path "%s" is not a valid dlfa-188_v1-indexer-http-requests-xml repo http-requests/ subdirectory`, args[2]))
	}

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

	if actualValue != goldenValue {
		err := writeActualSolrXMLToTmp(testEAD, fileID, actualValue)
		if err != nil {
			return fmt.Errorf("Error writing actual temp file for test case \"%s/%s\": %s",
				testEAD, fileID, err)
		}

		return fmt.Errorf("%s golden and actual values do not match\n", fileID)
	}

	return nil
}

func tmpFile(testEAD string, fileID string) string {
	return filepath.Join(tmpFilesDirPath, testEAD, fileID+goldenFileSuffix)
}

func usage() {
	println("usage: go run main.go [path to findingaids_eads_v2] [path to dlfa-188_v1-indexer-http-requests-xml/http-requests/]")
}

func writeActualSolrXMLToTmp(testEAD string, fileID string, actual string) error {
	tmpFile := tmpFile(testEAD, fileID)
	err := os.MkdirAll(filepath.Dir(tmpFile), 0755)
	if err != nil {
		return err
	}

	return os.WriteFile(tmpFile, []byte(actual), 0644)
}

func main() {
	setDirectoryPaths()

	err := clean()
	if err != nil {
		panic("clean() error: " + err.Error())
	}

	testEADs := getTestEADs()

	for _, testEAD := range testEADs {
		fmt.Println("Testing " + testEAD)
		eadXML, err := getEADValue(testEAD)
		if err != nil {
			println(fmt.Sprintf(`getEADValue("%s") failed: %s`, testEAD, err))
		}

		repositoryCode := parseRepositoryCode(testEAD)
		eadToTest, err := ead.New(repositoryCode, eadXML)
		if err != nil {

			println(fmt.Sprintf(`ead.New("%s", [EADXML for %s ]) failed: %s`, repositoryCode, testEAD, err))
		}

		err = testCollectionDocSolrAddMessage(testEAD, eadToTest.CollectionDoc.SolrAddMessage)
		if err != nil {
			println(err.Error())
		}

		if eadToTest.Components == nil {
			fmt.Println(testEAD + "has no components.  Skipping component tests")

			continue
		}

		componentIDs := []string{}
		for _, component := range *eadToTest.Components {
			componentIDs = append(componentIDs, component.ID)
			err = testComponentSolrAddMessage(testEAD, component.ID,
				component.SolrAddMessage)
			if err != nil {
				println(err.Error())
			}
		}

		err = testNoMissingComponents(testEAD, componentIDs)
		if err != nil {
			println(err.Error())
		}
	}
}
