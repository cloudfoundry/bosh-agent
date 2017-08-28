The certs in this directory will expire on 15 May 2018. Regenerate them by then using:

```
# /tmp/manifest.yml

---
variables:
- name: default_ca
  type: certificate
  options:
    is_ca: true
- name: my_keys
  type: certificate
  options:
    common_name: default.nats.bosh
    alternative_names: [ localhost ]
    ca: default_ca
- name: missing_cn
  type: certificate
  options:
    ca: default_ca
- name: invalid_cn
  type: certificate
  options:
    common_name: 123-456-789.invalid.bosh
    ca: default_ca
```

extract and save the values like

```
$ bosh int --vars-store creds.yml /tmp/manifest.yml
$ bosh int creds.yml --path /my_keys/certificate > custom_cert.pem
$ bosh int creds.yml --path /my_keys/private_key > custom_key.pem
$ bosh int creds.yml --path /my_keys/ca > custom_ca.pem
$ bosh int creds.yml --path /missing_cn/certificate > missing_cn_cert.pem
$ bosh int creds.yml --path /missing_cn/private_key > missing_cn_key.pem
$ bosh int creds.yml --path /invalid_cn/certificate > invalid_cn_cert.pem
$ bosh int creds.yml --path /invalid_cn/private_key > invalid_cn_key.pem
```