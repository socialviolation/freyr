

```shell
# https://sdk.operatorframework.io/docs/building-operators/golang/tutorial/
cd op_freyr

operator-sdk init --domain=freyr.fmtl.au --owner=nick@fmtl.au --project-name op-freyr

operator-sdk create api --group freyr --version v1alpha1 --kind Operator --resource --controller
```