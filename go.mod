module github.com/crazycs520/continuous-profile

go 1.16

require (
	github.com/dgraph-io/badger/v3 v3.2103.1
	github.com/genjidb/genji v0.13.0
	github.com/genjidb/genji/engine/badgerengine v0.13.0
	github.com/google/pprof v0.0.0-20210827144239-02619b876842
	github.com/gorilla/mux v1.8.0
	github.com/pingcap/errors v0.11.0
	github.com/pingcap/fn v0.0.0-20200306044125-d5540d389059
	github.com/pingcap/log v0.0.0-20210906054005-afc726e70354
	github.com/pkg/errors v0.9.1
	github.com/prometheus/common v0.30.0
	github.com/stretchr/testify v1.7.0
	github.com/vmihailenco/tagparser v0.1.2 // indirect
	go.opencensus.io v0.22.5 // indirect
	go.uber.org/zap v1.19.0
	golang.org/x/net v0.0.0-20210525063256-abc453219eb5
	gopkg.in/yaml.v2 v2.4.0
	honnef.co/go/tools v0.0.1-2020.1.4
)

replace github.com/dgraph-io/badger/v3 => github.com/crazycs520/badger/v3 v3.0.0-20210922063928-f25457a6a6fd
