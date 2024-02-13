
apply.all:
	kubectl apply -f svc_captain/manifest.yaml
	kubectl apply -f svc_conscript/manifest.yaml

docket:
	 curl http://34.117.50.242/docket | jq .;

enlist:
	 curl http://34.117.50.242/enlist

watch:
	watch --color --differences --no-wrap "curl http://localhost/docket | jq .;"




osdk.docker.build:
	cd op_freyr && make docker-build IMG=australia-southeast2-docker.pkg.dev/freyr-operator/imgs/freyr-operator:latest
osdk.docker.push:
	cd op_freyr && make docker-push IMG=australia-southeast2-docker.pkg.dev/freyr-operator/imgs/freyr-operator:latest
osdk.bundle:
	cd op_freyr && make bundle IMG=australia-southeast2-docker.pkg.dev/freyr-operator/imgs/freyr-operator:latest

k8s.apply:
	operator-sdk run -n operator-testing bundle australia-southeast2-docker.pkg.dev/freyr-operator/imgs/freyr-operator:latest

k8s.create-pull-secret:
	kubectl create secret docker-registry gcr-json-key \
		--docker-server=australia-southeast2-docker.pkg.dev \
		--docker-username=_json_key \
		--docker-password="$(cat secrets/freyr-operator-b9bde5075a35.json)"
	kubectl patch serviceaccount default -p '{"imagePullSecrets": [{"name": "gcr-json-key"}]}'
