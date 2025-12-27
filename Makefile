PKG:=github.com/hoon-x/rootweb
MODULE_NAME:=rootweb
VERSION:=0.9.0
COMMIT:=$(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_DATE:=$(shell date +%Y-%m-%d' '%H:%M:%S)

BIN_DIR:=bin
CONF_DIR:=config
ASSETS_DIR:=assets
CONF_FILE:=${MODULE_NAME}.yaml

LDFLAGS:=-X '${PKG}/config.Version=${VERSION}' \
		 -X '${PKG}/config.ModuleName=${MODULE_NAME}' \
		 -X '${PKG}/config.BuildDate=${BUILD_DATE}' \
		 -X '${PKG}/config.Commit=${COMMIT}'

# 디버그 모드 설정
ifeq ($(DEBUG),1)
	GCFLAGS:=-gcflags="all=-N -l"
	BUILD_TYPE:=DEBUG
else
	GCFLAGS:=
	BUILD_TYPE:=RELEASE
endif

# 빌드 매크로 정의
define go_build
	mkdir -p ${BIN_DIR}/${CONF_DIR}
	@echo ">> Building ${BUILD_TYPE} mode: ${MODULE_NAME}"
	go build ${GCFLAGS} -o ${BIN_DIR}/${MODULE_NAME} -ldflags "${LDFLAGS}"
	cp -f ${CONF_DIR}/${CONF_FILE} ${BIN_DIR}/${CONF_DIR}/${CONF_FILE}
	cp -rf ${ASSETS_DIR} ${BIN_DIR}/
endef

all: init build

init:
	@if [ ! -f go.mod ]; then \
		echo "Initalize ${MODULE_NAME}..."; \
		go mod init ${PKG}; \
	fi
	go mod tidy

build:
	$(call go_build)

debug:
	$(MAKE) build DEBUG=1

clean:
	rm -rf ${BIN_DIR}

.PHONY: init build debug clean
