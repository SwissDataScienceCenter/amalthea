package jupyterserver

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig"
	"github.com/SwissDataScienceCenter/amalthea/controllers/templates"
	jsonpatch "github.com/evanphx/json-patch"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/client-go/discovery"
	memory "k8s.io/client-go/discovery/cached"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	sigsYaml "sigs.k8s.io/yaml"
)

func SetupK8sClients() (*K8sClients, error) {
	var kubeconfig *string
	var err error
	kubeconfigFlagSet := flag.NewFlagSet("kubeconfig", flag.ExitOnError)
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = kubeconfigFlagSet.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = kubeconfigFlagSet.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	err = kubeconfigFlagSet.Parse(make([]string, 0))
	if err != nil {
		return nil, err
	}
	restConfig, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		return nil, err
	}
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	// DiscoveryClient queries API server about the resources
	dc, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(dc))

	output := K8sClients{
		DynamicClient:   dynamicClient,
		Kubeconfig:      *kubeconfig,
		DiscoveryClient: dc,
		RestConfig:      restConfig,
		RestMapper:      mapper,
		ClientSet:       clientset,
	}
	return &output, nil
}

func (js *JypterServerType) RenderTemplates() error {
	t := template.New("template").Funcs(sprig.TxtFuncMap())
	t = template.Must(t.ParseFS(templates.Templates, "*"))
	combinedJsonMap := make(map[string]json.RawMessage)
	for _, it := range t.Templates() {
		if it.Name() == "pvc.yaml" && !js.Manifest.Spec.Storage.Pvc.Enabled {
			// NOTE: If PVCs are disabled do not render the PVC manifest at all
			continue
		}
		renderedYaml := bytes.NewBufferString("")
		err := it.Execute(renderedYaml, js.Manifest)
		if err != nil {
			return err
		}
		renderedJson, err := sigsYaml.YAMLToJSON(renderedYaml.Bytes())
		if err != nil {
			return err
		}
		combinedJsonMap[strings.Replace(it.Name(), ".yaml", "", 1)] = renderedJson
	}
	output, err := json.Marshal(combinedJsonMap)
	if err != nil {
		return err
	}
	js.Templates = output
	return nil
}

// find the corresponding GVR (available in *meta.RESTMapping) for gvk
func (js *JypterServerType) findGVR(gvk *schema.GroupVersionKind) (*meta.RESTMapping, error) {
	return js.K8s.RestMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
}

func (js *JypterServerType) ApplyPatches() error {
	var parsedPatch jsonpatch.Patch
	var patchJson []byte
	var err error
	for _, patch := range js.Manifest.Spec.Patches {
		patchJson, err = sigsYaml.YAMLToJSON([]byte(patch.Patch))
		if err != nil {
			return err
		}
		switch patch.Type {
		case "application/json-patch+json":
			parsedPatch, err = jsonpatch.DecodePatch(patchJson)
			if err != nil {
				return err
			}
			js.Templates, err = parsedPatch.Apply(js.Templates)
			if err != nil {
				return err
			}
		case "application/merge-patch+json":
			js.Templates, err = jsonpatch.MergePatch(js.Templates, patchJson)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (js *JypterServerType) GetUnstructuredResources() error {
	var err error
	var jsonResources map[string]json.RawMessage
	err = json.Unmarshal(js.Templates, &jsonResources)
	if err != nil {
		return err
	}
	dec := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	for _, jsonResource := range jsonResources {
		obj := &unstructured.Unstructured{}

		_, _, err := dec.Decode(jsonResource, nil, obj)
		if err != nil {
			return err
		}
		js.Children = append(js.Children, obj)
	}
	return nil
}

func (js *JypterServerType) CreateResources(resources []unstructured.Unstructured) error {
	for _, res := range resources {
		gvk := res.GetObjectKind().GroupVersionKind()
		resMapping, err := js.findGVR(&gvk)
		if err != nil {
			return err
		}
		_, err = js.K8s.DynamicClient.Resource(resMapping.Resource).Namespace(js.Manifest.Namespace).Create(context.TODO(), &res, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func (js *JypterServerType) GetMissingResources() ([]unstructured.Unstructured, error) {
	output := make([]unstructured.Unstructured, 0, 5)
	for _, res := range js.Children {
		gvk := res.GetObjectKind().GroupVersionKind()
		resMapping, err := js.findGVR(&gvk)
		if err != nil {
			return nil, err
		}
		_, err = js.K8s.DynamicClient.Resource(resMapping.Resource).Namespace(js.Manifest.Namespace).Get(context.TODO(), js.Manifest.Name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				output = append(output, *res)
			} else {
				return nil, err
			}
		}
	}
	return output, nil
}
