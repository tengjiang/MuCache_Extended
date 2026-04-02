GO      = /usr/local/go/bin/go
GOFLAGS =
BINDIR  = bin

CHAIN_SERVICES = service1 service2 service3 service4 backend

.PHONY: all chain clean

all: chain

chain: $(addprefix $(BINDIR)/chain_,$(addsuffix _nocm,$(CHAIN_SERVICES)))

$(BINDIR)/chain_%_nocm:
	@mkdir -p $(BINDIR)
	$(GO) build $(GOFLAGS) -o $@ ./cmd/chain/$*

clean:
	rm -rf $(BINDIR)
