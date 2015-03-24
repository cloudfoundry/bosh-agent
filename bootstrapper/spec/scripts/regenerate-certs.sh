#!/bin/bash
# from http://datacenteroverlords.com/2012/03/01/creating-your-own-ssl-certificate-authority/

certDir=`dirname $0`/certs
rm -rf $certDir
mkdir -p $certDir
cd $certDir

function createConfig () { organizationName=$1
  cat > /tmp/cert.cfg <<EOF
[ req ]
prompt             = no
distinguished_name = req_distinguished_name

[ req_distinguished_name ]
organizationName = ${organizationName}

[ my-extensions ]
extendedKeyUsage = clientAuth,serverAuth
subjectAltName = @alt_names

[alt_names]
DNS.1 = *

EOF
}

function cleanup () { name=$1
  rm ${name}.csr
  rm /tmp/cert.cfg
}

function createSigningRequest () { name=$1; organizationName=$2
  createConfig $organizationName
  openssl genrsa -out ${name}.key 1024
  openssl req -new -key ${name}.key -out ${name}.csr -config /tmp/cert.cfg -extensions my-extensions
}

function createCertWithCA () { name=$1; organizationName=$2
  echo "Generating #{organizationName} cert signed by CA..."
  createSigningRequest $name $organizationName
  openssl x509 -req -in ${name}.csr -CA rootCA.pem -CAkey rootCA.key -CAcreateserial -out ${name}.crt -days 99999 -extfile /tmp/cert.cfg -extensions my-extensions
  cleanup $name
}

function createSelfSignedCert () { name=$1; organizationName=$2
  echo "Generating self-signed #{organizationName} cert..."
  createSigningRequest $name $organizationName
  openssl x509 -req -in ${name}.csr -signkey ${name}.key -out ${name}.crt -days 99999 -extfile /tmp/cert.cfg -extensions my-extensions
  cleanup $name
}

echo "Generating CA..."
openssl genrsa -out rootCA.key 1024
yes "" | openssl req -x509 -new -nodes -key rootCA.key -days 99999 -out rootCA.pem

createCertWithCA bootstrapper bosh.bootstrapper
createCertWithCA director bosh.director
createSelfSignedCert directorWithWrongCA bosh.director

echo "Done!"