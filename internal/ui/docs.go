package ui

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/bjarneo/kli/internal/k8s"
)

var kubernetesDocs = map[string]string{
	"pods":                            "https://kubernetes.io/docs/concepts/workloads/pods/",
	"deployments":                     "https://kubernetes.io/docs/concepts/workloads/controllers/deployment/",
	"statefulsets":                    "https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/",
	"daemonsets":                      "https://kubernetes.io/docs/concepts/workloads/controllers/daemonset/",
	"replicasets":                     "https://kubernetes.io/docs/concepts/workloads/controllers/replicaset/",
	"replicationcontrollers":          "https://kubernetes.io/docs/concepts/workloads/controllers/replicationcontroller/",
	"jobs":                            "https://kubernetes.io/docs/concepts/workloads/controllers/job/",
	"cronjobs":                        "https://kubernetes.io/docs/concepts/workloads/controllers/cron-jobs/",
	"services":                        "https://kubernetes.io/docs/concepts/services-networking/service/",
	"ingresses":                       "https://kubernetes.io/docs/concepts/services-networking/ingress/",
	"networkpolicies":                 "https://kubernetes.io/docs/concepts/services-networking/network-policies/",
	"endpoints":                       "https://kubernetes.io/docs/concepts/services-networking/service/#endpoints",
	"endpointslices":                  "https://kubernetes.io/docs/concepts/services-networking/endpoint-slices/",
	"configmaps":                      "https://kubernetes.io/docs/concepts/configuration/configmap/",
	"secrets":                         "https://kubernetes.io/docs/concepts/configuration/secret/",
	"serviceaccounts":                 "https://kubernetes.io/docs/concepts/security/service-accounts/",
	"persistentvolumes":               "https://kubernetes.io/docs/concepts/storage/persistent-volumes/",
	"persistentvolumeclaims":          "https://kubernetes.io/docs/concepts/storage/persistent-volumes/#persistentvolumeclaims",
	"storageclasses":                  "https://kubernetes.io/docs/concepts/storage/storage-classes/",
	"namespaces":                      "https://kubernetes.io/docs/concepts/overview/working-with-objects/namespaces/",
	"nodes":                           "https://kubernetes.io/docs/concepts/architecture/nodes/",
	"events":                          "https://kubernetes.io/docs/reference/kubernetes-api/cluster-resources/event-v1/",
	"horizontalpodautoscalers":        "https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/",
	"poddisruptionbudgets":            "https://kubernetes.io/docs/tasks/run-application/configure-pdb/",
	"resourcequotas":                  "https://kubernetes.io/docs/concepts/policy/resource-quotas/",
	"limitranges":                     "https://kubernetes.io/docs/concepts/policy/limit-range/",
	"customresourcedefinitions":       "https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/",
	"mutatingwebhookconfigurations":   "https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/",
	"validatingwebhookconfigurations": "https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/",
}

func kubernetesDocsURL(res k8s.ResourceInfo) (string, bool) {
	u, ok := kubernetesDocs[res.Resource]
	return u, ok
}

func (a App) docsResource() (k8s.ResourceInfo, bool) {
	res := a.res
	switch a.screen {
	case screenConfig:
		if a.configTarget.res.Resource != "" {
			res = a.configTarget.res
		}
	case screenDetail:
		if a.detailTarget.res.Resource != "" {
			res = a.detailTarget.res
		}
	case screenLogs:
		res = k8s.ResourceInfo{Resource: "pods", Kind: "Pod", Namespaced: true}
	case screenCockpit:
		return k8s.ResourceInfo{}, false
	}
	if res.Resource == "" {
		return k8s.ResourceInfo{}, false
	}
	return res, true
}

func (a App) openDocs() (tea.Model, tea.Cmd) {
	res, ok := a.docsResource()
	if !ok {
		a.setStatus("docs: no Kubernetes docs for this view", true)
		return a, nil
	}
	u, ok := kubernetesDocsURL(res)
	if !ok {
		a.setStatus("docs: no Kubernetes docs for "+res.Resource, true)
		return a, nil
	}
	return a, openDocsCmd(u)
}

func openDocsCmd(u string) tea.Cmd {
	return func() tea.Msg {
		if err := openBrowser(u); err != nil {
			return statusMsg{text: "docs: " + err.Error(), err: true}
		}
		return statusMsg{text: "opened docs: " + u}
	}
}

func openBrowser(raw string) error {
	if err := validateDocsURL(raw); err != nil {
		return err
	}
	name, args := browserCommand(raw)
	if name == "" {
		return fmt.Errorf("no browser opener found")
	}
	return exec.Command(name, args...).Start()
}

func validateDocsURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("invalid docs url")
	}
	return nil
}

func browserCommand(raw string) (string, []string) {
	switch runtime.GOOS {
	case "linux":
		if browser := strings.Fields(os.Getenv("BROWSER")); len(browser) > 0 {
			return browser[0], append(browser[1:], raw)
		}
		return "xdg-open", []string{raw}
	case "darwin":
		return "open", []string{raw}
	case "windows":
		return "rundll32", []string{"url.dll,FileProtocolHandler", raw}
	}
	return "", nil
}
