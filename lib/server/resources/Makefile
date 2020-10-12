GO?=go

.PHONY:	clean generate vet

DIRECTORIES := $(sort $(dir $(wildcard */)))

all: generate

generate:
	@(cd enums && $(MAKE) $(@))
	@(cd operations && $(MAKE) $(@))
	@$(GO) $(@)

vet:
	@$(GO) vet ./...

clean:
	@(cd enums && $(MAKE) $(@))
	@(cd operations && $(MAKE) $(@))
