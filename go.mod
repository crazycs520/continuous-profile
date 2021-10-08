module github.com/crazycs520/continuous-profile

go 1.16

require (
	cloud.google.com/go v0.50.0 // indirect
	cloud.google.com/go/bigquery v1.3.0 // indirect
	cloud.google.com/go/pubsub v1.1.0 // indirect
	cloud.google.com/go/storage v1.4.0 // indirect
	github.com/dgraph-io/badger/v3 v3.2103.1
	github.com/envoyproxy/go-control-plane v0.9.4 // indirect
	github.com/genjidb/genji v0.13.0
	github.com/genjidb/genji/engine/badgerengine v0.13.0
	github.com/golang/groupcache v0.0.0-20200121045136-8c9f03a8e57e // indirect
	github.com/golang/mock v1.4.4 // indirect
	github.com/google/martian/v3 v3.0.0 // indirect
	github.com/google/pprof v0.0.0-20210827144239-02619b876842
	github.com/gorilla/mux v1.8.0
	github.com/jstemmer/go-junit-report v0.9.1 // indirect
	github.com/pingcap/errors v0.11.5-0.20200917111840-a15ef68f753d
	github.com/pingcap/fn v0.0.0-20200306044125-d5540d389059
	github.com/pingcap/log v0.0.0-20210906054005-afc726e70354
	github.com/pingcap/tidb-dashboard v0.0.0-20211008050453-a25c25809529
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.11.0 // indirect
	github.com/prometheus/common v0.26.0
	github.com/stretchr/testify v1.7.0
	github.com/vmihailenco/tagparser v0.1.2 // indirect
	go.etcd.io/etcd v0.5.0-alpha.5.0.20191023171146-3cf2f69b5738
	go.opencensus.io v0.22.5 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	go.uber.org/fx v1.10.0 // indirect
	go.uber.org/zap v1.19.0
	golang.org/x/exp v0.0.0-20200224162631-6cc2880d07d6 // indirect
	golang.org/x/net v0.0.0-20210525063256-abc453219eb5
	golang.org/x/time v0.0.0-20191024005414-555d28b269f0 // indirect
	google.golang.org/api v0.15.1 // indirect
	google.golang.org/appengine v1.6.6 // indirect
	google.golang.org/grpc v1.25.1
	gopkg.in/yaml.v2 v2.4.0
	honnef.co/go/tools v0.0.1-2020.1.4 // indirect
	rsc.io/quote/v3 v3.1.0 // indirect
)

replace github.com/dgraph-io/badger/v3 => github.com/crazycs520/badger/v3 v3.0.0-20210922063928-f25457a6a6fd
