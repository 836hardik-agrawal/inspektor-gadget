SHELL := /bin/bash

GADGET_TAG ?= $(shell ../tools/image-tag branch)
GADGET_REPOSITORY ?= ghcr.io/inspektor-gadget/gadget
BUILDER_IMAGE ?= ghcr.io/inspektor-gadget/ebpf-builder:latest
IG ?= ig
COSIGN ?= cosign
GADGETS = \
	trace_dns \
	trace_exec \
	trace_lsm \
	trace_malloc \
	trace_mount \
	trace_oomkill \
	trace_open \
	trace_signal \
	trace_sni \
	trace_ssl \
	trace_tcp \
	trace_tcpconnect \
	trace_tcpdrop \
	trace_tcpretrans \
	top_file \
	snapshot_process \
	snapshot_socket \
	ci/sched_cls_drop \
	#

.PHONY: all
all: build

build: $(GADGETS)

# GADGET_BUILD_PARAMS can be used to pass additional parameters e.g
# GADGET_BUILD_PARAMS="--update-metadata" make build
.PHONY: $(GADGETS)
$(GADGETS):
	@echo "Building $@"
	@sudo -E IG_EXPERIMENTAL=true $(IG) image build \
		--builder-image $(BUILDER_IMAGE) \
		-t $(GADGET_REPOSITORY)/$@:$(GADGET_TAG) \
		$$GADGET_BUILD_PARAMS \
		$@

%-push: %
	@echo "Pushing $*"
	@sudo -E IG_EXPERIMENTAL=true $(IG) image push $(GADGET_REPOSITORY)/$*:$(GADGET_TAG)

.PHONY:
push: $(addsuffix -push,$(GADGETS))

%-sign: %-push
	@echo "Signing $*"
	digest=$$(sudo -E IG_EXPERIMENTAL=true $(IG) image list --no-trunc | grep "$* " | awk '{ print $$3 }') ; \
	cosign sign --key env://COSIGN_PRIVATE_KEY --yes --recursive $(GADGET_REPOSITORY)/$*@$$digest

sign: $(addsuffix -sign,$(GADGETS))

%-clean:
	sudo -E IG_EXPERIMENTAL=true $(IG) image remove $(GADGET_REPOSITORY)/$*:$(GADGET_TAG)

.PHONY:
clean: $(addsuffix -clean,$(GADGETS))

.PHONY:
test: build
	CGO_ENABLED=0 \
	IG_EXPERIMENTAL=true \
	GADGET_REPOSITORY=$(GADGET_REPOSITORY) \
	GADGET_TAG=$(GADGET_TAG) \
	IG=$(IG) \
	go test -exec 'sudo -E' -v ./...
