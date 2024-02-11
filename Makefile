
apply.all:
	kubectl apply -f svc_captain/manifest.yaml
	kubectl apply -f svc_conscript/manifest.yaml

docket:
	 curl http://34.117.50.242/docket | jq .;

enlist:
	 curl http://34.117.50.242/enlist

watch:
	watch --color --differences --no-wrap "curl http://34.117.50.242/docket | jq .;"
