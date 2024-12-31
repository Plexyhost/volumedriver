PLUGIN_NAME=yourusername/volumedriver
PLUGIN_TAG=latest

all: clean rootfs create

clean:
	@echo "### rm ./plugin"
	@rm -rf ./plugin

rootfs:
	@echo "### docker build rootfs image"
	@docker build -t ${PLUGIN_NAME}:rootfs -f Dockerfile.plugin .
	@echo "### create rootfs directory"
	@mkdir -p ./plugin/rootfs
	@docker create --name tmp ${PLUGIN_NAME}:rootfs
	@docker export tmp | tar -x -C ./plugin/rootfs
	@echo "### copy config.json"
	@cp config.json ./plugin/
	@docker rm -vf tmp

create:
	@echo "### create new plugin ${PLUGIN_NAME}:${PLUGIN_TAG}"
	@docker plugin create ${PLUGIN_NAME}:${PLUGIN_TAG} ./plugin

enable:
	@echo "### enable plugin ${PLUGIN_NAME}:${PLUGIN_TAG}"
	@docker plugin enable ${PLUGIN_NAME}:${PLUGIN_TAG}

push:
	@echo "### push plugin ${PLUGIN_NAME}:${PLUGIN_TAG}"
	@docker plugin push ${PLUGIN_NAME}:${PLUGIN_TAG}
