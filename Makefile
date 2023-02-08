IMG_BUILDER := $(shell command -v docker || command -v podman)
TEST_C_DIR := test/containers

build-test-images:
	@echo "Creating the test images..."
	cd $(TEST_C_DIR)/error && $(IMG_BUILDER) build -t atk-errer .
	cd $(TEST_C_DIR)/paramlist && $(IMG_BUILDER) build -t atk-lister .
	cd $(TEST_C_DIR)/paramvalidate && $(IMG_BUILDER) build -t atk-validator .

	cd $(TEST_C_DIR)/predeploy && $(IMG_BUILDER) build -t atk-predeployer .
	cd $(TEST_C_DIR)/deploy && $(IMG_BUILDER) build -t atk-deployer .
	cd $(TEST_C_DIR)/postdeploy && $(IMG_BUILDER) build -t atk-postdeployer .

test-all: build-test-images
	@ITZ_PODMAN_PATH=$(IMG_BUILDER) go test github.com/cloud-native-toolkit/atkmod/test

build-all:
	go build

# To use the same target as other projects.
ci: test-all
