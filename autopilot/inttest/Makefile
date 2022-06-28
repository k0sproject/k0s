AUTOPILOT_PATH = $(realpath ../autopilot)
LOGS_BASE ?= $(realpath logs)

integrationtests := \
	check-airgap \
	check-ha3x3 \
	check-platformselect \
	check-nocontrollers \
	check-quorum \
	check-quorumsafety \
	check-selector \
	check-single \
	check-updater \



.PHONY: all
all: $(integrationtests)

.PHONY: clean
clean:
	-rm .footloose-alpine.stamp
	-rm .update-server.stamp

.footloose-alpine.stamp: footloose-alpine/Dockerfile
	docker build -t footloose-autopilot-alpine -f $< $(dir $<)
	touch $@

.PHONY: $(integrationtests)
$(integrationtests): .footloose-alpine.stamp
	mkdir -p $(LOGS_BASE)/$@
	AUTOPILOT_PATH=$(AUTOPILOT_PATH) \
	AUTOPILOT_LOG_DIR=$(realpath logs/)/$@ \
	go test \
	    -timeout=20m -count=1 -v \
	    github.com/k0sproject/autopilot/inttest/tests/$(subst check-,,$@)


.update-server.stamp: update-server/Dockerfile update-server/html/stable/index.yaml
	docker build -t update-server --build-arg BASE=footloose-autopilot-alpine -f $< $(dir $<)
	touch $@

.PHONY: check-updater
check-updater: .footloose-alpine.stamp .update-server.stamp
	mkdir -p $(LOGS_BASE)/$@
	AUTOPILOT_PATH=$(AUTOPILOT_PATH) \
	AUTOPILOT_LOG_DIR=$(realpath logs/)/$@ \
	go test \
	    -timeout=20m -count=1 -v \
	    github.com/k0sproject/autopilot/inttest/tests/updater
