# dlfa-250-set-up-all-ead-test-for-go-ead-indexer-package

Jira: [Set up all EAD test for go-ead-indexer`ead` package](https://jira.nyu.edu/browse/DLFA-250)

Run the golden files test:

```bash
dlfa-250-set-up-all-ead-test-for-go-ead-indexer-package/> ./diff.sh \
> [RELATIVE OR ABSOLUTE PATH]/findingaids_eads_v2 \
> [RELATIVE OR ABSOLUTE PATH]/dlfa-188_v1-indexer-http-requests-xml/http-requests

real    52m25.082s
user    47m13.581s
sys     4m15.262s
dlfa-250-set-up-all-ead-test-for-go-ead-indexer-package/>  
```

Outputs:

* _diffs/_: results of `diff [GOLDEN FILE] [ACTUAL FILE]` for each golden file
 if the diff is not empty.
* _logs/_: datetime-stamped stdout and stderr logs for the test run.
* _tmp/actual/_: actual files for test failures.

