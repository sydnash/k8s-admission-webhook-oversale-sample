CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o webhook-server .
if [[ $? == 0 ]]; then echo "successfully build..."; else echo "build failed!!! exiting..."; exit 1; fi
image="webhook-server:`date +%Y%m%d%H%M%S`"
docker build -t ${image} .
kubectl patch deploy pod-annotate-webhook --type=json -p="[{\"op\": \"replace\", \"path\": \"/spec/template/spec/containers/0/image\", \"value\": \"${image}\"}]"
 
