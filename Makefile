ndef = $(if $(value $(1)),,$(error $(1) not set))

.DEFAULT_GOAL := help

.PHONY: default
default: help ;

#ROOTDIR:=$(shell ROOTDIR='$(ROOTDIR)' bash -c "dirname $(realpath $(lastword $(MAKEFILE_LIST)))")
#export ROOTDIR

include ./common.mk

# Binaries generated
EXECS=cli/safescale/safescale cli/safescale/safescale-cover cli/safescaled/safescaled cli/safescaled/safescaled-cover cli/scanner/scanner

# List of files
PKG_FILES := $(shell find . \( -path ./vendor -o -path ./Godeps \) -prune -o -type f -name '*.go' -print | grep -v version.go | grep -v gomock_reflect_ )
# List of packages
PKG_LIST := $(shell $(GO) list ./... | grep -v lib/security/ | grep -v /vendor/)
# List of packages to test
TESTABLE_PKG_LIST := $(shell $(GO) list ./... | grep -v /vendor/ | grep -v lib/security/ | grep -v providers/aws | grep -v stacks/aws | grep -v sandbox)

# Get go version
GOVER := $(shell $(GO) version | awk '{print $$3}')

# DEPENDENCIES MANAGEMENT
STRINGER := golang.org/x/tools/cmd/stringer
RICE := github.com/GeertJohan/go.rice github.com/GeertJohan/go.rice/rice
PROTOC := github.com/golang/protobuf
PROTOBUF := github.com/golang/protobuf/protoc-gen-go

# Build tools
CONVEY := github.com/smartystreets/goconvey
MOCKGEN := github.com/golang/mock/gomock github.com/golang/mock/mockgen
COVER := golang.org/x/tools/cmd/cover
LINTER := golang.org/x/lint/golint
DEP := github.com/golang/dep/cmd/dep
ERRCHECK := github.com/kisielk/errcheck
XUNIT := github.com/tebeka/go2xunit
COVERTOOL := github.com/dlespiau/covertool
GOVENDOR := github.com/kardianos/govendor

DEVDEPSLIST := $(RICE) $(PROTOBUF) $(DEP) $(COVER) $(XUNIT) $(COVERTOOL) $(GOVENDOR)

BUILD_TAGS :=
export BUILD_TAGS

all: begin ground getdevdeps ensure generate lib cli err vet-light
	@printf "%b" "$(OK_COLOR)$(OK_STRING) Build SUCCESSFUL $(NO_COLOR)\n";

common: begin ground getdevdeps ensure generate

versioncut:
	@(($(GO) version | grep go1.12) || ($(GO) version | grep go1.13) || ($(GO) version | grep go1.14) || ($(GO) version | grep go1.15)) || (printf "%b" "$(ERROR_COLOR)$(ERROR_STRING) Minimum go version is 1.12 ! $(NO_COLOR)\n" && /bin/false);

begin: versioncut
	@printf "%b" "$(OK_COLOR)$(INFO_STRING) Build begins ...$(NO_COLOR)\n";
	@printf "%b" "$(OK_COLOR)$(INFO_STRING) Go: $(GOVER) ...$(NO_COLOR)\n";
	@printf "%b" "$(OK_COLOR)$(INFO_STRING) Gopath: $(GOPATH) ...$(NO_COLOR)\n";
	@printf "%b" "$(OK_COLOR)$(INFO_STRING) Gobin: $(GOBIN) ...$(NO_COLOR)\n";

libvirt:
	@printf "%b" "$(OK_COLOR)$(INFO_STRING) Libvirt driver enabled$(NO_COLOR)\n";
	@systemctl status libvirtd.service >/dev/null 2>&1 || { printf "%b" "$(ERROR_COLOR)$(ERROR_STRING) libvirt is required but it's not installed.  Aborting.$(NO_COLOR)\n" >&2; exit 1; }
	@lsmod | grep kvm >/dev/null 2>&1 || { printf "%b" "$(ERROR_COLOR)$(ERROR_STRING) kvm is required but it's not installed.  Aborting.$(NO_COLOR)\n" >&2; exit 1; }
	@grep -E '^flags.*(vmx|svm)' /proc/cpuinfo >/dev/null 2>&1 && \
	if [ $$? -eq 0 ]; then \
		printf "%b" "$(OK_COLOR)$(OK_STRING) Hardware acceleration is available!\n"; \
	else \
		printf "%b" "$(WARN_COLOR)$(WARN_STRING) Hardware acceleration is NOT available!\n"; \
	fi
	@$(eval BUILD_TAGS = "--tags=libvirt")
	@export BUILD_TAGS="--tags=libvirt"

with_git:
	@command -v git >/dev/null 2>&1 || { printf "%b" "$(ERROR_COLOR)$(ERROR_STRING) git is required but it's not installed.  Aborting.$(NO_COLOR)\n" >&2; exit 1; }

ground:
	@printf "%b" "$(OK_COLOR)$(INFO_STRING) Testing tool prerequisites, $(NO_COLOR)target $(OBJ_COLOR)$(@)$(NO_COLOR)\n";
	@command -v git >/dev/null 2>&1 || { printf "%b" "$(ERROR_COLOR)$(ERROR_STRING) git is required but it's not installed.  Aborting.$(NO_COLOR)\n" >&2; exit 1; }
	@command -v $(GO) >/dev/null 2>&1 || { printf "%b" "$(ERROR_COLOR)$(ERROR_STRING) go is required but it's not installed.  Aborting.$(NO_COLOR)\n" >&2; exit 1; }
	@command -v protoc >/dev/null 2>&1 || { printf "%b" "$(ERROR_COLOR)$(ERROR_STRING) protoc is required but it's not installed.  Aborting.$(NO_COLOR)\n" >&2; exit 1; }

getdevdeps: begin
	@printf "%b" "$(OK_COLOR)$(INFO_STRING) Testing prerequisites, $(NO_COLOR)target $(OBJ_COLOR)$(@)$(NO_COLOR)\n";
	@which dep > /dev/null; if [ $$? -ne 0 ]; then \
    	printf "%b" "$(OK_COLOR)$(INFO_STRING) Downloading dep...\n" && $(GO) get -u $(DEP); \
    fi
	@which rice > /dev/null; if [ $$? -ne 0 ]; then \
		printf "%b" "$(OK_COLOR)$(INFO_STRING) Downloading rice...\n" && $(GO) get -u $(RICE); \
	fi
	@which govendor > /dev/null; if [ $$? -ne 0 ]; then \
		printf "%b" "$(OK_COLOR)$(INFO_STRING) Downloading govendor...\n" && $(GO) get -u $(GOVENDOR); \
	fi
	@which cover covertool > /dev/null; if [ $$? -ne 0 ]; then \
		printf "%b" "$(OK_COLOR)$(INFO_STRING) Downloading cover and covertool...\n" && $(GO) get -u $(COVER) $(COVERTOOL); \
	fi
	@which go2xunit > /dev/null; if [ $$? -ne 0 ]; then \
		printf "%b" "$(OK_COLOR)$(INFO_STRING) Downloading go2xunit...\n" && $(GO) get -u $(XUNIT); \
	fi
	@which mockgen > /dev/null; if [ $$? -ne 0 ]; then \
		printf "%b" "$(OK_COLOR)$(INFO_STRING) Downloading mockgen...\n" && $(GO) get -u $(MOCKGEN); \
	fi
	@which errcheck > /dev/null; if [ $$? -ne 0 ]; then \
		printf "%b" "$(OK_COLOR)$(INFO_STRING) Downloading errcheck...\n" && $(GO) get -u $(ERRCHECK); \
	fi
	@which goconvey > /dev/null; if [ $$? -ne 0 ]; then \
  		mkdir ./vendor >/dev/null 2>&1 || true; \
		printf "%b" "$(OK_COLOR)$(INFO_STRING) Downloading convey...\n" && govendor fetch $(CONVEY)@v1.6.3 && GOBIN=$(GOPATH)/bin $(GO) install vendor/github.com/smartystreets/goconvey/goconvey.go; \
	fi
	@which golint > /dev/null; if [ $$? -ne 0 ]; then \
		printf "%b" "$(OK_COLOR)$(INFO_STRING) Downloading linter...\n" && $(GO) get -u $(LINTER); \
	fi
	@which stringer > /dev/null; if [ $$? -ne 0 ]; then \
		printf "%b" "$(OK_COLOR)$(INFO_STRING) Downloading stringer...\n" && $(GO) get -u $(STRINGER); \
	fi
	@which golangci-lint > /dev/null; if [ $$? -ne 0 ]; then \
  		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(GOPATH)/bin v1.26.0; \
	fi

ensure:
	@printf "%b" "$(OK_COLOR)$(INFO_STRING) Checking versions, $(NO_COLOR)target $(OBJ_COLOR)$(@)$(NO_COLOR)\n";
	@if [ ! -d ./vendor ]; then printf "%b" "$(OK_COLOR)$(INFO_STRING) Downloading all dependencies from zero, this is gonna take a while..., $(NO_COLOR)target $(OBJ_COLOR)$(@)$(NO_COLOR)\n"; else printf "%b" "$(OK_COLOR)$(INFO_STRING) Updating vendor dir..., $(NO_COLOR)target $(OBJ_COLOR)$(@)$(NO_COLOR)\n"; fi;
	@while [ 1 -ne 0 ] ; do \
		$$(dep ensure) && break || printf "%b" "$(OK_COLOR)$(INFO_STRING) timeout resolving dependencies, retrying..., $(NO_COLOR)target $(OBJ_COLOR)$(@)$(NO_COLOR)\n"; \
	done
	@printf "%b" "$(OK_COLOR)$(INFO_STRING) Installing protobuf... $(NO_COLOR)target $(OBJ_COLOR)$(@)$(NO_COLOR)\n";
	@(govendor get github.com/golang/protobuf/protoc-gen-go@1.2.0 && GOBIN=$(GOPATH)/bin $(GO) install ./vendor/github.com/golang/protobuf/protoc-gen-go)

sdk:
	@(cd lib && $(MAKE) $(@))

lib: common
	@printf "%b" "$(OK_COLOR)$(INFO_STRING) Building SafeScale libraries, $(NO_COLOR)target $(OBJ_COLOR)$(@)$(NO_COLOR)\n";
	@(cd lib && $(MAKE) all)

cli: common lib
	@printf "%b" "$(OK_COLOR)$(INFO_STRING) Building SafeScale binaries, $(NO_COLOR)target $(OBJ_COLOR)$(@)$(NO_COLOR)\n";
	@(cd cli && $(MAKE) all)

clean:
	@printf "%b" "$(OK_COLOR)$(INFO_STRING) Cleaning..., $(NO_COLOR)target $(OBJ_COLOR)$(@)$(NO_COLOR)\n";
	@(cd cli && $(MAKE) $(@))
	@(cd lib && $(MAKE) $(@))

mrproper: clean
	@(git clean -xdf -e .idea -e vendor -e .vscode || true)

install:
	@($(CP) -f $(EXECS) $(GOBIN) || true)

installci:
	@(mkdir -p $(CIBIN) || true)
	@($(CP) -f $(EXECS) $(CIBIN) || true)

godocs:
	@printf "%b" "$(OK_COLOR)$(INFO_STRING) Running godocs in background, $(NO_COLOR)target $(OBJ_COLOR)$(@)$(NO_COLOR)\n";
	@(godoc -http=:6060 &)

convey:
	@printf "%b" "$(OK_COLOR)$(INFO_STRING) Running goconvey in background, $(NO_COLOR)target $(OBJ_COLOR)$(@)$(NO_COLOR)\n";
	@(cd lib && goconvey -port 8082 &)

conveystop:
	@printf "%b" "$(OK_COLOR)$(INFO_STRING) Stopping goconvey in background, $(NO_COLOR)target $(OBJ_COLOR)$(@)$(NO_COLOR)\n";
	@(ps -ef | grep goconvey | grep 8082 | awk {'print $2'} | xargs kill -9 || true)

depclean: begin
	@printf "%b" "$(OK_COLOR)$(INFO_STRING) Cleaning vendor and redownloading deps, $(NO_COLOR)target $(OBJ_COLOR)$(@)$(NO_COLOR)\n";
	@if [ -f ./Gopkg.lock ]; then rm ./Gopkg.lock; fi;
	@rm -rf ./vendor
	@if [ ! -d ./vendor ]; then printf "%b" "$(OK_COLOR)$(INFO_STRING) Downloading all dependencies from zero, this is gonna take a while..., $(NO_COLOR)target $(OBJ_COLOR)$(@)$(NO_COLOR)\n"; else printf "%b" "$(OK_COLOR)$(INFO_STRING) Updating vendor dir..., $(NO_COLOR)target $(OBJ_COLOR)$(@)$(NO_COLOR)\n"; fi;
	@while [ 1 -ne 0 ] ; do \
		$$(dep ensure) && break || printf "%b" "$(OK_COLOR)$(INFO_STRING) timeout resolving dependencies, retrying..., $(NO_COLOR)target $(OBJ_COLOR)$(@)$(NO_COLOR)\n"; \
	done
	@printf "%b" "$(OK_COLOR)$(INFO_STRING) Installing protobuf... $(NO_COLOR)target $(OBJ_COLOR)$(@)$(NO_COLOR)\n";
	@($(GO) install ./vendor/github.com/golang/protobuf/protoc-gen-go)
	@printf "%b" "$(OK_COLOR)$(INFO_STRING) Updating gophercloud... $(NO_COLOR)target $(OBJ_COLOR)$(@)$(NO_COLOR)\n";
	@while [ 1 -ne 0 ] ; do \
		$$(dep ensure -update "github.com/gophercloud/gophercloud") && break || printf "%b" "$(OK_COLOR)$(INFO_STRING) timeout resolving dependencies, retrying..., $(NO_COLOR)target $(OBJ_COLOR)$(@)$(NO_COLOR)\n"; \
	done
	@printf "%b" "$(OK_COLOR)$(INFO_STRING) Updating stow... $(NO_COLOR)target $(OBJ_COLOR)$(@)$(NO_COLOR)\n";
	@while [ 1 -ne 0 ] ; do \
		$$(dep ensure -update "github.com/graymeta/stow") && break || printf "%b" "$(OK_COLOR)$(INFO_STRING) timeout resolving dependencies, retrying..., $(NO_COLOR)target $(OBJ_COLOR)$(@)$(NO_COLOR)\n"; \
	done

generate: begin # Run generation
	@printf "%b" "$(OK_COLOR)$(INFO_STRING) Running code generation, $(NO_COLOR)target $(OBJ_COLOR)$(@)$(NO_COLOR)\n";
	@cd lib && $(MAKE) generate 2>&1 | tee -a generation_results.log
	@cd cli && $(MAKE) generate 2>&1 | tee -a generation_results.log
	@if [ -z "$(GOOS)" ]; then $(GO) generate -run mockgen ./...  2>&1 | tee -a generation_results.log;fi;
	@if [ -s ./generation_results.log ]; then printf "%b" "$(WARN_COLOR)$(WARN_STRING) Warning generating code, if RICE related, then is a false warning !$(NO_COLOR)\n";fi;

test: begin # Run unit tests
	@printf "%b" "$(OK_COLOR)$(INFO_STRING) Running unit tests, $(NO_COLOR)target $(OBJ_COLOR)$(@)$(NO_COLOR)\n";
	@$(GO) test -v ${PKG_LIST} 2>&1 > test_results.log || true
	@go2xunit -input test_results.log -output xunit_tests.xml || true
	@if [ -s ./test_results.log ]; then printf "%b" "$(ERROR_COLOR)$(ERROR_STRING) tests FAILED ! Take a look at ./test_results.log $(NO_COLOR)\n";else printf "%b" "$(OK_COLOR)$(OK_STRING) CONGRATS. TESTS PASSED ! $(NO_COLOR)\n";fi;

test-light: begin # Run unit tests
	@printf "%b" "$(OK_COLOR)$(INFO_STRING) Running unit tests (with restrictions), $(NO_COLOR)target $(OBJ_COLOR)$(@)$(NO_COLOR)\n";
	@$(GO) test -v ${TESTABLE_PKG_LIST} 2>&1 > test_results.log || true
	@go2xunit -input test_results.log -output xunit_tests.xml || true
	@if [ -s ./test_results.log ]; then printf "%b" "$(ERROR_COLOR)$(ERROR_STRING) tests (with restrictions) FAILED ! Take a look at ./test_results.log $(NO_COLOR)\n";exit 1;else printf "%b" "$(OK_COLOR)$(OK_STRING) CONGRATS. TESTS PASSED ! $(NO_COLOR)\n";fi;

gofmt: begin
	@printf "%b" "$(OK_COLOR)$(INFO_STRING) Running gofmt checks, $(NO_COLOR)target $(OBJ_COLOR)$(@)$(NO_COLOR)\n";
	@if [ -n "$$($(GOFMT) -d $(PKG_FILES))" ]; then \
		"$$($(GOFMT) -d $(PKG_FILES))" \
		echo "-- gofmt check failed"; \
		/bin/false; \
	fi

vet-light: begin
	@printf "%b" "$(OK_COLOR)$(INFO_STRING) Running vet checks (with restrictions), $(NO_COLOR)target $(OBJ_COLOR)$(@)$(NO_COLOR)\n";
	@$(GO) vet ${TESTABLE_PKG_LIST} 2>&1 | tee vet_results.log
	@if [ -s ./vet_results.log ]; then printf "%b" "$(ERROR_COLOR)$(ERROR_STRING) vet (with restrictions) FAILED !$(NO_COLOR)\n";exit 1;else printf "%b" "$(OK_COLOR)$(OK_STRING) CONGRATS. NO PROBLEMS DETECTED ! $(NO_COLOR)\n";fi;

err: begin
	@printf "%b" "$(OK_COLOR)$(INFO_STRING) Running errcheck, $(NO_COLOR)target $(OBJ_COLOR)$(@)$(NO_COLOR)\n";
	@errcheck ${TESTABLE_PKG_LIST} 2>&1 | grep -v _test | grep -v test_ | tee err_results.log
	@if [ -s ./err_results.log ]; then printf "%b" "$(ERROR_COLOR)$(ERROR_STRING) errcheck (with restrictions) FAILED !$(NO_COLOR)\n";exit 1;else printf "%b" "$(OK_COLOR)$(OK_STRING) CONGRATS. NO PROBLEMS DETECTED ! $(NO_COLOR)\n";fi;

err-light: begin
	@printf "%b" "$(OK_COLOR)$(INFO_STRING) Running errcheck (with restrictions), $(NO_COLOR)target $(OBJ_COLOR)$(@)$(NO_COLOR)\n";
	@errcheck ${TESTABLE_PKG_LIST} 2>&1 | grep -v defer | grep -v _test | grep -v test_ | tee err_results.log
	@if [ -s ./err_results.log ]; then printf "%b" "$(ERROR_COLOR)$(ERROR_STRING) errcheck (with restrictions) FAILED !$(NO_COLOR)\n";exit 1;else printf "%b" "$(OK_COLOR)$(OK_STRING) CONGRATS. NO PROBLEMS DETECTED ! $(NO_COLOR)\n";fi;

vet: begin
	@printf "%b" "$(OK_COLOR)$(INFO_STRING) Running vet checks, $(NO_COLOR)target $(OBJ_COLOR)$(@)$(NO_COLOR)\n";
	@$(GO) vet ${PKG_LIST} 2>&1 | tee vet_results.log
	@if [ -s ./vet_results.log ]; then printf "%b" "$(ERROR_COLOR)$(ERROR_STRING) vet FAILED !$(NO_COLOR)\n";exit 1;else printf "%b" "$(OK_COLOR)$(OK_STRING) CONGRATS. NO PROBLEMS DETECTED ! $(NO_COLOR)\n";fi

lint: begin
	@printf "%b" "$(OK_COLOR)$(INFO_STRING) Running lint checks, $(NO_COLOR)target $(OBJ_COLOR)$(@)$(NO_COLOR)\n";
	@golint ./... | grep -v vendor | grep -v test | grep -v Test | grep -v enum\. | grep -v version\.go || true

metalint: begin
	@printf "%b" "$(OK_COLOR)$(INFO_STRING) Running metalint checks, $(NO_COLOR)target $(OBJ_COLOR)$(@)$(NO_COLOR)\n";
	@(which golangci-lint && golangci-lint --color never --disable-all --enable=deadcode --enable=gocyclo --enable=varcheck --enable=structcheck --enable=maligned --enable=errcheck --enable=ineffassign --enable=interfacer --enable=unconvert --enable=goconst --enable=gosec --enable=megacheck --enable=gocritic --enable=depguard run --enable=dogsled --enable=funlen --enable=gochecknoglobals --deadline=8m ./... || true) || echo "golangci-lint not installed in your system"

style: begin generate gofmt
	@printf "%b" "$(OK_COLOR)$(INFO_STRING) Running style checks, $(NO_COLOR)target $(OBJ_COLOR)$(@)$(NO_COLOR)\n";
	@(which golangci-lint > /dev/null && golangci-lint --color never --disable-all --enable=errcheck --enable=stylecheck --enable=deadcode --enable=golint --enable=gocritic --enable=staticcheck --enable=gosimple --enable=govet --enable=ineffassign --enable=varcheck --deadline=8m run ./... || true) || echo "golangci-lint not installed in your system"

coverage: begin generate
	@printf "%b" "$(OK_COLOR)$(INFO_STRING) Collecting coverage data, $(NO_COLOR)target $(OBJ_COLOR)$(@)$(NO_COLOR)\n";
	@$(GO) test -v ${TESTABLE_PKG_LIST} -coverprofile=cover.out > coverage_results.log 2>&1 || true
	@$(GO) tool cover -html=cover.out -o cover.html || true

show-cov: begin
	@command -v firefox >/dev/null 2>&1 || { printf "%b" "$(ERROR_COLOR)$(ERROR_STRING) You don't have firefox on PATH.  Aborting.$(NO_COLOR)\n" >&2; exit 1; }
	@if [ -s ./cover.out ]; then $(GO) tool cover -html=cover.out -o cover.html || true;fi
	@if [ -s ./cover.html ]; then $(BROWSER) ./cover.html || true;fi

logclean: begin
	@printf "%b" "$(OK_COLOR)$(INFO_STRING) Cleaning logs... $(NO_COLOR)target $(OBJ_COLOR)$(@)$(NO_COLOR)\n";
	@$(RM) test_results.log || true
	@$(RM) coverage_results.log || true
	@$(RM) generation_results.log || true
	@$(RM) vet_results.log || true
	@$(RM) xunit_tests.xml || true
	@$(RM) cover.out || true
	@$(RM) cover.html || true

status: with_git
	@git remote update >/dev/null 2>&1
	@printf "%b" "$(WARN_COLOR)LOCAL BUILD STATUS:$(NO_COLOR)\n";
	@printf "%b" "$(NO_COLOR)  Build hash $(OK_COLOR)$(BUILD)$(GOLD_COLOR)$(NO_COLOR)\n";
	@printf "%b" "$(WARN_COLOR)";
	@if [ $(LOCAL) = $(REMOTE) ]; then echo "  Build Up-to-date"; elif [ $(LOCAL) = $(BASE) ]; then echo "  You are behind origin/develop"; elif [ $(REMOTE) = $(BASE) ]; then echo "  You have local commits NOT PUSHED to origin/develop"; else echo "  Build Diverged, you have to merge"; fi
	@printf "%b" "$(NO_COLOR)";

help: with_git
	@echo ''
	@git remote update >/dev/null 2>&1
	@printf "%b" "$(GOLD_COLOR) *************** SAFESCALE BUILD$(GOLD_COLOR) ****************$(NO_COLOR)\n";
	@echo ' If in doubt, try "make all"'
	@echo ''
	@printf "%b" "$(OK_COLOR)BUILD TARGETS:$(NO_COLOR)\n";
	@printf "%b" "  $(GOLD_COLOR)all          - Builds all binaries$(NO_COLOR)\n";
	@printf "%b" "$(NO_COLOR)";
	@echo '  help         - Prints this help message'
	@echo '  godocs       - Runs godoc in background at port 6060.'
	@echo '                 Go to (http://localhost:6060/pkg/github.com/CS-SI/)'
	@echo '  install      - Copies all binaries to $(GOBIN)'
	@echo ''
	@printf "%b" "$(OK_COLOR)TESTING TARGETS:$(NO_COLOR)\n";
	@printf "%b" "$(NO_COLOR)";
	@echo '  lint         - Runs linter'
	@echo '  metalint     - Runs golangci-lint'
	@echo '  vet          - Runs all checks'
	@echo '  vet-light    - Runs all checks (with restrictions)'
	@echo '  err          - Looks for unhandled errors'
	@echo '  err-light    - Looks for unhandled errors (with restrictions)'
	@echo '  test         - Runs all tests'
	@echo '  test-light   - Runs all tests (with restrictions)'
	@echo '  convey       - Runs goconvey in lib dir'
	@echo '  coverage     - Collects coverage info from unit tests'
	@echo '  show-cov     - Displays coverage info in firefox'
	@echo ''
	@printf "%b" "$(OK_COLOR)DEV TARGETS:$(NO_COLOR)\n";
	@printf "%b" "$(NO_COLOR)";
	@echo '  clean        - Removes files generated by build.'
	@echo '  depclean     - Rebuilds vendor dependencies'
	@echo '  logclean     - Removes log files generated by build.'
	@echo '  status       - Shows build status.'
	@echo ''
	@echo
