# GooseEngine — build
#
# The UCI engine is cmd/engine. There is no main package at the repo root, so
# the old `go build -o $(EXE)` here could never have worked.
#
# PEXT is chosen at *runtime*, not at build time: goosemg.hasFastPEXT() reads
# CPUID and only takes the hardware PEXTQ path on CPUs that implement PEXT in
# silicon (Intel Haswell+, AMD Zen 3+); everything else falls back to pextSoft.
# No build flag turns that on or off, and none should.
#
# What a build flag *does* control is GOAMD64 — the microarchitecture level the
# compiler is allowed to assume. v3 is the Haswell/Zen 2 baseline (AVX2,
# BMI1/BMI2, FMA, LZCNT, POPCNT, MOVBE), which lets math/bits calls and the
# runtime's memmove emit those instructions directly instead of routing through
# runtime feature checks. That is the right companion to the fast-PEXT path —
# same CPU generation — but a v3 binary dies with SIGILL on anything older, so
# it is auto-detected for native builds and explicit for cross builds.

PKG    := ./cmd/engine
BINDIR := bin
NAME   := chess-engine

GO      ?= go
GOFLAGS ?=
LDFLAGS ?= -s -w
TAGS    ?=

HOST_GOOS   := $(shell $(GO) env GOOS)
HOST_GOARCH := $(shell $(GO) env GOARCH)

EXT :=
ifeq ($(HOST_GOOS),windows)
  EXT := .exe
endif

# --- Host AVX2 detection -> GOAMD64 level ------------------------------------
# Only meaningful when the host is amd64; on arm64 the toolchain ignores
# GOAMD64 entirely, so it is left unset there. Detecting avx2 is a proxy for
# the whole v3 feature set — every CPU that has AVX2 has the rest of it.
# Override at any time: `GOAMD64=v1 make`.
AVX2 :=
ifeq ($(HOST_GOARCH),amd64)
  ifeq ($(HOST_GOOS),darwin)
    AVX2 := $(shell sysctl -n machdep.cpu.leaf7_features 2>/dev/null | tr 'A-Z' 'a-z' | grep -qw avx2 && echo yes)
  else
    AVX2 := $(shell grep -qiw avx2 /proc/cpuinfo 2>/dev/null && echo yes)
  endif
  ifeq ($(AVX2),yes)
    GOAMD64 ?= v3
  else
    GOAMD64 ?= v1
  endif
  export GOAMD64
endif

# Cross-compiled amd64 artifacts. v3 by default (AVX2 + hardware PEXT machines);
# `make release CROSS_GOAMD64=v1` for a binary that runs on pre-Haswell CPUs.
CROSS_GOAMD64 ?= v3

# Go needs an in-repo temp dir under MSYS (see AGENTS.md); no-op elsewhere.
GOTMP :=
ifeq ($(HOST_GOOS),windows)
  GOTMP := TMPDIR="$(CURDIR)/.hermes-tmp"
endif

BUILD = $(GOTMP) $(GO) build $(GOFLAGS) $(if $(TAGS),-tags "$(TAGS)") -ldflags "$(LDFLAGS)"

.PHONY: all engine portable windows linux release test bench info clean

all: engine

engine: | $(BINDIR)
	$(BUILD) -o $(BINDIR)/$(NAME)$(EXT) $(PKG)

# Same host target, but pinned to the generic amd64 baseline.
portable:
	GOAMD64=v1 $(MAKE) engine NAME=$(NAME)-portable

windows: | $(BINDIR)
	GOOS=windows GOARCH=amd64 GOAMD64=$(CROSS_GOAMD64) \
		$(BUILD) -o $(BINDIR)/$(NAME)-windows-amd64-$(CROSS_GOAMD64).exe $(PKG)

linux: | $(BINDIR)
	GOOS=linux GOARCH=amd64 GOAMD64=$(CROSS_GOAMD64) \
		$(BUILD) -o $(BINDIR)/$(NAME)-linux-amd64-$(CROSS_GOAMD64) $(PKG)

release: windows linux

test:
	$(GOTMP) $(GO) test ./engine/ ./goosemg/ ./tests/ ./tuner/

bench:
	$(GOTMP) $(GO) test -run '^$$' -bench . ./bench/

info:
	@echo "host        : $(HOST_GOOS)/$(HOST_GOARCH)"
	@echo "avx2 (host) : $(if $(AVX2),yes,no/na)"
	@echo "GOAMD64     : $(if $(GOAMD64),$(GOAMD64),unset (non-amd64 host))"
	@echo "output      : $(BINDIR)/$(NAME)$(EXT)"
	@echo "note        : PEXT is selected at runtime by goosemg.hasFastPEXT()"

$(BINDIR):
	mkdir -p $(BINDIR)

clean:
	rm -f $(BINDIR)/$(NAME) $(BINDIR)/$(NAME)-* $(BINDIR)/$(NAME).exe
	rm -f $(NAME) $(NAME).exe
