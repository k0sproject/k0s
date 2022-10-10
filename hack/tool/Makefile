ifndef HOST_ARCH
HOST_HARDWARE := $(shell uname -m)
ifneq (, $(filter $(HOST_HARDWARE), aarch64 arm64 armv8l))
  HOST_ARCH := arm64
else ifneq (, $(filter $(HOST_HARDWARE), armv7l arm))
  HOST_ARCH := arm
else
  ifeq (, $(filter $(HOST_HARDWARE), x86_64 amd64 x64))
    $(warning unknown machine hardware name $(HOST_HARDWARE), assuming amd64)
  endif
  HOST_ARCH := amd64
endif
endif

image:
	docker build --build-arg ARCH=${HOST_ARCH} --build-arg HARDWARE=${HOST_HARDWARE} -t tool -f ./Dockerfile .
