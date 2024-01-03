#!/bin/bash

WD="$(dirname $0)/../certs"
HOST="localhost"
PORT=8443

CURL="curl -v --cert ${WD}/client.crt --key ${WD}/client.key --cacert ${WD}/ca.crt -H \"Content-Type: application/json\""

PASSED () {
    echo -e "\033[1m\033[42mPASSED\033[0m"
}
FAILED () {
    echo -e "\033[1m\033[41mFAILED\033[0m"
}


echo -n 'Test /pingz..... '
${CURL} -X GET https://${HOST}:${PORT}/api/v1/pingz 2>&1  | grep -q '{"status":"alive"}'
if [ $? -eq 0 ] ; then PASSED; else FAILED; fi

echo -n 'Test wrong cert type....'
curl --cert "${WD}/server.crt" --key "${WD}/server.key" --cacert "${WD}/ca.crt" -X GET https://${HOST}:${PORT}/api/v1/pingz -H "Content-Type: application/json" 2>&1 | grep -q 'bad certificate'
if [ $? -eq 0 ] ; then PASSED; else FAILED; fi

echo -n 'Test no cert connection...'
curl --cacert "${WD}/ca.crt" -X GET https://${HOST}:${PORT}/api/v1/pingz -H "Content-Type: application/json" 2>&1 | grep -q 'certificate required'
if [ $? -eq 0 ] ; then PASSED; else FAILED; fi

echo -n 'Test /healthz....'
${CURL} -X GET https://${HOST}:${PORT}/api/v1/healthz 2>&1 | grep -qE '{"status":"(un)?healthy"'
if [ $? -eq 0 ] ; then PASSED; else FAILED; fi

echo -n 'Test /deployments....'
${CURL} -X GET https://${HOST}:${PORT}/api/v1/deployments 2>&1 | grep -q '"name":"sre-server-deployment"'
if [ $? -eq 0 ] ; then PASSED; else FAILED; fi

echo -n 'Test replicas GET...'
${CURL} -X GET https://${HOST}:${PORT}/api/v1/namespaces/default/deployments/sre-server-deployment/replicas 2>&1 | grep -q '"name":"sre-server-deployment","replicaCount":1'
if [ $? -eq 0 ] ; then PASSED; else FAILED; fi

echo -n 'Test replicas PUT...'
${CURL} -X PUT https://${HOST}:${PORT}/api/v1/namespaces/default/deployments/sre-server-deployment/replicas -d '{"replicaCount": 3}' 2>&1 | grep -q '"name":"sre-server-deployment","replicaCount":3'
if [ $? -eq 0 ] ; then PASSED; else FAILED; fi

sleep 1

echo -n 'Test new replicaCount...'
${CURL} -X GET https://${HOST}:${PORT}/api/v1/namespaces/default/deployments/sre-server-deployment/replicas 2>&1 | grep -q '"name":"sre-server-deployment","replicaCount":3'
if [ $? -eq 0 ] ; then PASSED; else FAILED; fi

echo -n 'Set replicaCount backto 1...'
${CURL} -X PUT https://${HOST}:${PORT}/api/v1/namespaces/default/deployments/sre-server-deployment/replicas -d '{"replicaCount": 1}' 2>&1 | grep -q '"name":"sre-server-deployment","replicaCount":1'
if [ $? -eq 0 ] ; then PASSED; else FAILED; fi
