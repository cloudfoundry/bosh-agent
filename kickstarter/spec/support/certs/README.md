Here's what you do: (cribbed from http://datacenteroverlords.com/2012/03/01/creating-your-own-ssl-certificate-authority/ )

```
openssl genrsa -out rootCA.key 1024
yes "" | openssl req -x509 -new -nodes -key rootCA.key -days 99999 -out rootCA.pem

openssl genrsa -out director.key 1024
yes "" | openssl req -new -key director.key -out director.csr
openssl x509 -req -in director.csr -CA rootCA.pem -CAkey rootCA.key -CAcreateserial -out director.crt -days 99999 -extfile cert.cfg -extensions 'director'
```


``` - self signed director cert
openssl genrsa -out directorWithWrongCA.key 1024
yes "" | openssl req -new -key directorWithWrongCA.key -out directorWithWrongCA.csr
openssl x509 -req -in directorWithWrongCA.csr -signkey directorWithWrongCA.key -out directorWithWrongCA.crt -days 99999 -extfile cert.cfg -extensions 'director'
```