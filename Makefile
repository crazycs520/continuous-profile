
PROJECT=continuous-profile
PACKAGE_LIST  := go list ./...| grep -E "github.com\/crazycs520\/continuous-profile\/"
PACKAGES  ?= $$($(PACKAGE_LIST))
PACKAGE_DIRECTORIES := $(PACKAGE_LIST) | sed 's|github.com/crazycs520/$(PROJECT)/||'
FILES     := $$(find $$($(PACKAGE_DIRECTORIES)) -name "*.go")
FAIL_ON_STDOUT := awk '{ print } END { if (NR > 0) { exit 1 } }'

GO              := GO111MODULE=on go
GOBUILD         := $(GO) build


default:
	$(GOBUILD) -o bin/conprof ./main.go
	@echo Build successfully!

fmt:
	@echo "gofmt (simplify)"
	@gofmt -s -l -w . 2>&1 | $(FAIL_ON_STDOUT)
	@gofmt -s -l -w $(FILES) 2>&1 | $(FAIL_ON_STDOUT)
