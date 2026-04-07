GO      = /usr/local/go/bin/go
GOFLAGS =
BINDIR  = bin

CHAIN_SERVICES = service1 service2 service3 service4 backend
HOTEL_SERVICES = frontend search rate profile reservation user

.PHONY: all chain hotel clean

all: chain hotel

# ── chain benchmark ────────────────────────────────────────────────────────────

chain: $(addprefix $(BINDIR)/chain_,\
	$(addsuffix _nocm,  $(CHAIN_SERVICES)) \
	$(addsuffix _flame, $(CHAIN_SERVICES)))

$(BINDIR)/chain_%_nocm:
	@mkdir -p $(BINDIR)
	$(GO) build $(GOFLAGS) -o $@ ./cmd/chain/$*

$(BINDIR)/chain_%_flame:
	@mkdir -p $(BINDIR)
	$(GO) build $(GOFLAGS) -tags flame -o $@ ./cmd/chain/$*

# ── hotel benchmark ────────────────────────────────────────────────────────────

hotel: $(addprefix $(BINDIR)/hotel_,\
	$(addsuffix _nocm,  $(HOTEL_SERVICES)) \
	$(addsuffix _flame, $(HOTEL_SERVICES)))

$(BINDIR)/hotel_%_nocm:
	@mkdir -p $(BINDIR)
	$(GO) build $(GOFLAGS) -o $@ ./cmd/hotel/$*

$(BINDIR)/hotel_%_flame:
	@mkdir -p $(BINDIR)
	$(GO) build $(GOFLAGS) -tags flame -o $@ ./cmd/hotel/$*

clean:
	rm -rf $(BINDIR)
