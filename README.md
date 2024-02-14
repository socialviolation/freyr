# Freyr

Freyr is an example golang Kubernetes Operator using operator-sdk.

```shell
# https://sdk.operatorframework.io/docs/building-operators/golang/tutorial/
cd op_freyr

operator-sdk init --domain=freyr.fmtl.au --owner=nick@fmtl.au --project-name op-freyr

operator-sdk create api --group freyr --version v1alpha1 --kind Operator --resource --controller
```


## Resources:
* [Operator SDK Go](https://docs.okd.io/latest/operators/operator_sdk/golang/osdk-golang-tutorial.html#osdk-run-operator_osdk-golang-tutorial)
* [KIND SA IMG PULL](https://colinwilson.uk/2020/07/09/using-google-container-registry-with-kubernetes/#step-3---grant-the-service-account-permissions)

## TODO:
* [ ] freyr - inject spec into config map
* [ ] inject apikey into k8s secret
* [ ] captain - read in config, and pub the spec to response
* [ ] improve docs
* [ ] tf automation
