```shell
gcloud container clusters update operator-cluster --region=australia-southeast2 --update-addons=HttpLoadBalancing=ENABLED
```


```yaml
apiVersion: networking.k8s.io/v1
kind: IngressClass
metadata:
  name: gce
  annotations:
    ingressclass.kubernetes.io/is-default-class: "true"
spec:
  controller: k8s.io/ingress-gce

---
```