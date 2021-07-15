module github.com/openshift/must-gather-operator

go 1.13

require (
	github.com/coreos/prometheus-operator v0.38.0
	github.com/go-logr/logr v0.1.0
	github.com/go-openapi/spec v0.19.4
	github.com/openshift/api v3.9.1-0.20190924102528-32369d4db2ad+incompatible
	github.com/openshift/operator-custom-metrics v0.3.1-0.20200901174648-463079905232
	github.com/operator-framework/operator-sdk v0.17.2
	github.com/prometheus/client_golang v1.5.1
	github.com/redhat-cop/operator-utils v0.1.0
	github.com/scylladb/go-set v1.0.2
	github.com/spf13/pflag v1.0.5
	k8s.io/api v0.17.4
	k8s.io/apimachinery v0.17.4
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/kube-openapi v0.0.0-20191107075043-30be4d16710a
	sigs.k8s.io/controller-runtime v0.5.2
)

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v13.3.2+incompatible // Required by OLM
	k8s.io/client-go => k8s.io/client-go v0.17.4 // Required by prometheus-operator
)
