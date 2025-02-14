# dlfa-250-set-up-all-ead-test-for-go-ead-indexer-package

Jira: [Set up all EAD test for go-ead-indexer`ead` package](https://jira.nyu.edu/browse/DLFA-250)

Setup:

```bash
git clone git@github.com:NYULibraries/dlfa-250-set-up-all-ead-test-for-go-ead-indexer-package.git
cd dlfa-250-set-up-all-ead-test-for-go-ead-indexer-package/
ln -s [PATH TO CLONE OF https://github.com/NYULibraries/go-ead-indexer]
```

Run the golden files test:

```bash
dlfa-250-set-up-all-ead-test-for-go-ead-indexer-package/> ./diff.sh \
> [RELATIVE OR ABSOLUTE PATH]/findingaids_eads_v2 \
> [RELATIVE OR ABSOLUTE PATH]/dlfa-188_v1-indexer-http-requests/http-requests
 
real    79m45.204s
user    72m48.342s
sys     8m48.870s
dlfa-250-set-up-all-ead-test-for-go-ead-indexer-package/>  
```

Outputs:

* _diffs/_: results of `diff [GOLDEN FILE] [ACTUAL FILE]` for each golden file
 if the diff is not empty.
* _logs/_: datetime-stamped stdout and stderr logs for the test run.
* _tmp/actual/_: actual files for test failures.

-----

# Versions of repos used for current diffs 

* go-ead-indexer: [see go.mod file]
* dlfa-188_v1-indexer-http-requests: [206386a464e2b1280021571cbd4e73218990c26c](https://github.com/NYULibraries/dlfa-188_v1-indexer-http-requests/tree/206386a464e2b1280021571cbd4e73218990c26c)
* findingaids_eads_v2: [8d1b8fb6bd45327e90857c77bff8afa66358f4e7](https://github.com/NYULibraries/findingaids_eads_v2/tree/8d1b8fb6bd45327e90857c77bff8afa66358f4e7)
