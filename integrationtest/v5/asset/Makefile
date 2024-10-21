BYBIT_TEST_UPDATED=false

test:
	BYBIT_TEST_UPDATED=${BYBIT_TEST_UPDATED} BYBIT_TEST_KEY=${BYBIT_TEST_KEY} BYBIT_TEST_SECRET=${BYBIT_TEST_SECRET} go test github.com/Issengaard/bybit_sdk/integrationtest/v5/asset -v -tags=integrationtestv5asset

test-spec:
	BYBIT_TEST_UPDATED=${BYBIT_TEST_UPDATED} BYBIT_TEST_KEY=${BYBIT_TEST_KEY} BYBIT_TEST_SECRET=${BYBIT_TEST_SECRET} go test github.com/Issengaard/bybit_sdk/integrationtest/v5/asset -v -tags=integrationtestv5asset -run ${BYBIT_TEST_METHOD}

