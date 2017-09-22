#!/bin/bash

bosh int --vars-store creds.yml test_cert_manifest.yml
bosh int creds.yml --path /my_keys/certificate > custom_cert.pem
bosh int creds.yml --path /my_keys/private_key > custom_key.pem
bosh int creds.yml --path /my_keys/ca > custom_ca.pem
bosh int creds.yml --path /missing_cn/certificate > missing_cn_cert.pem
bosh int creds.yml --path /missing_cn/private_key > missing_cn_key.pem
bosh int creds.yml --path /invalid_cn/certificate > invalid_cn_cert.pem
bosh int creds.yml --path /invalid_cn/private_key > invalid_cn_key.pem
