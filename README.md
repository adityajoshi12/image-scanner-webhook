# image-scanner-webhook
This project implements a Kubernetes mutating webhook that adds an init container to pods. This init container performs container image scanning of all the containers within the pod using `snyk` scanning tool.
NOTE: This is a general example and you will need to adapt it to your specific requirements and chosen image scanning tool.

## Deploy

### TLS Certs
#### CA Certificates
```bash
openssl genrsa -out ca.key 2048
openssl req -new -x509 -days 365 -key ca.key -subj "/C=CN/ST=GD/L=SZ/O=Acme, Inc./CN=Acme Root CA" -out ca.crt
```
#### Issue TLS certificates
TLS certifcates for `resource-webhook` service
```bash
export SERVICE=image-scanner
export NAMESPACE=webhook
openssl req -newkey rsa:2048 -nodes -keyout tls.key -subj "/C=CN/ST=GD/L=SZ/O=Acme, Inc./CN=$SERVICE.$NAMESPACE.svc.cluster.local" -out tls.csr
openssl x509 -req -extfile <(printf "subjectAltName=DNS:$SERVICE.$NAMESPACE.svc.cluster.local,DNS:$SERVICE.$NAMESPACE.svc.cluster,DNS:$SERVICE.$NAMESPACE.svc,DNS:$SERVICE.$NAMESPACE.svc,DNS:$SERVICE.$NAMESPACE,DNS:$SERVICE") -days 365 -in tls.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out tls.crt
```

#### Create TLS kubernetes Secret
```bash
kubectl create ns webhook
kubectl create secret tls tls --cert=tls.crt --key=tls.key -n webhook
```
#### Webhook Server
update the webhook-server.yaml with the `SNYK TOKEN`. You can get it from [snyk.io](https://app.snyk.io/)
```bash
kubectl apply -f deploy/webhook-server.yaml
```

#### Webhook Configuration
```bash
CA_CERT=$(cat ca.crt | base64)
sed -e 's@CA-CERT@'"$CA_CERT"'@g' <"deploy/webhook-template.yaml" > deploy/webhook.yaml
kubectl apply -f deploy/webhook.yaml
```

## Usage
Once deployed, the webhook will automatically:
- Intercept pod creation requests: The webhook will listen for pod creation events.
- Add an init container: The webhook will add an init container to the pod's spec.
- Scan container images: The init container will execute the image scanner and scan all container images within the pod.
- Proceed with pod creation: The webhook will allow the pod to be created once the image scanning is complete.

