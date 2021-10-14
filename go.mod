module github.com/crazycs520/continuous-profile

go 1.16

require (
	github.com/dgraph-io/badger/v3 v3.2103.1
	github.com/genjidb/genji v0.13.0
	github.com/genjidb/genji/engine/badgerengine v0.13.0
	github.com/golang/groupcache v0.0.0-20200121045136-8c9f03a8e57e // indirect
	github.com/gorilla/mux v1.8.0
	github.com/pingcap/errors v0.11.5-0.20200917111840-a15ef68f753d
	github.com/pingcap/log v0.0.0-20210906054005-afc726e70354
	github.com/pingcap/tidb-dashboard v0.0.0-20211008050453-a25c25809529
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.11.0 // indirect
	github.com/prometheus/common v0.26.0
	github.com/stretchr/testify v1.7.0
	go.etcd.io/etcd v0.5.0-alpha.5.0.20191023171146-3cf2f69b5738
	go.uber.org/atomic v1.9.0
	go.uber.org/fx v1.10.0
	go.uber.org/zap v1.19.0
	golang.org/x/net v0.0.0-20210525063256-abc453219eb5
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45 // indirect
	golang.org/x/time v0.0.0-20191024005414-555d28b269f0 // indirect
	google.golang.org/appengine v1.6.6 // indirect
	google.golang.org/genproto v0.0.0-20191216164720-4f79533eabd1 // indirect
	gopkg.in/yaml.v2 v2.4.0
)

replace github.com/dgraph-io/badger/v3 => github.com/crazycs520/badger/v3 v3.0.0-20210922063928-f25457a6a6fd

replace github.com/pingcap/tidb-dashboard => github.com/crazycs520/tidb-dashboard v0.0.0-20211009060758-44122db89f8c
