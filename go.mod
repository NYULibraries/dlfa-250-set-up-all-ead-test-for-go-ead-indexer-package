module dlfa_250_set_up_all_ead_test_for_go_ead_indexer_package

go 1.23.1

//require go-ead-indexer v0.0.0
//
//replace go-ead-indexer v0.0.0 => ./go-ead-indexer

require github.com/nyulibraries/go-ead-indexer v0.0.0-20250304031702-c3003a4434fe

require (
	github.com/lestrrat-go/libxml2 v0.0.0-20240905100032-c934e3fcb9d3 // indirect
	github.com/pkg/errors v0.9.1 // indirect
)
