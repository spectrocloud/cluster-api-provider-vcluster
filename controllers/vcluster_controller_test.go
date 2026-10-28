package controllers

import (
	"context"
	"encoding/json"
	"io"
	"k8s.io/apimachinery/pkg/labels"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1alpha1 "github.com/loft-sh/cluster-api-provider-vcluster/api/v1alpha1"
	"github.com/loft-sh/cluster-api-provider-vcluster/pkg/util/conditions"
)

const (
	testName      = "vcluster-test"
	testNamespace = "cluster-test"
)

func newTestVCluster(annotations map[string]string) *v1alpha1.VCluster {
	return &v1alpha1.VCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:        testName,
			Namespace:   testNamespace,
			Annotations: annotations,
		},
	}
}

// fakeAPIServer serves the minimal set of endpoints lifecycle.PauseVCluster needs
// and records the statefulset patch it receives.
type fakeAPIServer struct {
	mu                sync.Mutex
	patch             []byte
	namespaceSelector string
	cleanupError      bool
	pods              []corev1.Pod
	deleted           []string
	podListError      bool
	podDeleteError    bool
	podListCalls      int
	listErrorOn       int
}

func (f *fakeAPIServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	sts := appsv1.StatefulSet{
		TypeMeta:   metav1.TypeMeta{APIVersion: "apps/v1", Kind: "StatefulSet"},
		ObjectMeta: metav1.ObjectMeta{Name: testName, Namespace: testNamespace},
	}

	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/apis/apps/v1/namespaces/"+testNamespace+"/statefulsets":
		one := int32(1)
		sts.Spec.Replicas = &one
		_ = json.NewEncoder(w).Encode(appsv1.StatefulSetList{
			TypeMeta: metav1.TypeMeta{APIVersion: "apps/v1", Kind: "StatefulSetList"},
			Items:    []appsv1.StatefulSet{sts},
		})
	case r.Method == http.MethodPatch && r.URL.Path == "/apis/apps/v1/namespaces/"+testNamespace+"/statefulsets/"+testName:
		body, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.patch = body
		f.mu.Unlock()
		_ = json.NewEncoder(w).Encode(sts)
	case r.Method == http.MethodGet && r.URL.Path == "/apis/apps/v1/namespaces/"+testNamespace+"/statefulsets/"+testName:
		// report the statefulset as scaled down
		_ = json.NewEncoder(w).Encode(sts)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/namespaces/"+testNamespace+"/pods":
		f.mu.Lock()
		f.podListCalls++
		failList := f.podListError || f.podListCalls == f.listErrorOn
		f.mu.Unlock()
		if failList {
			http.Error(w, "pod list failed", http.StatusForbidden)
			return
		}
		selector, err := labels.Parse(r.URL.Query().Get("labelSelector"))
		if err != nil || r.URL.Query().Get("labelSelector") != "vcluster.loft.sh/managed-by="+testName {
			http.Error(w, "incorrect workload selector", http.StatusBadRequest)
			return
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		items := []corev1.Pod{}
		for _, pod := range f.pods {
			if selector.Matches(labels.Set(pod.Labels)) {
				items = append(items, pod)
			}
		}
		_ = json.NewEncoder(w).Encode(corev1.PodList{TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "PodList"}, Items: items})
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/v1/namespaces/"+testNamespace+"/pods/"):
		if f.podDeleteError {
			http.Error(w, "pod deletion failed", http.StatusForbidden)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/api/v1/namespaces/"+testNamespace+"/pods/")
		f.mu.Lock()
		defer f.mu.Unlock()
		for i, pod := range f.pods {
			if pod.Name == name {
				f.pods = append(f.pods[:i], f.pods[i+1:]...)
				f.deleted = append(f.deleted, name)
				break
			}
		}
		_ = json.NewEncoder(w).Encode(metav1.Status{TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Status"}, Status: "Success"})
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/namespaces":
		f.mu.Lock()
		f.namespaceSelector = r.URL.Query().Get("labelSelector")
		f.mu.Unlock()
		if f.cleanupError || r.URL.Query().Get("labelSelector") != "vcluster.loft.sh/managed-by=cluster-test-x-vcluster-test" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(metav1.Status{TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Status"}, Status: "Failure", Reason: metav1.StatusReasonBadRequest, Code: http.StatusBadRequest, Message: "namespace cleanup rejected"})
			return
		}
		_ = json.NewEncoder(w).Encode(corev1.NamespaceList{
			TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "NamespaceList"},
		})
	default:
		http.Error(w, "unexpected request "+r.Method+" "+r.URL.Path, http.StatusNotFound)
	}
}

func TestPauseIfNeededNotPaused(t *testing.T) {
	for name, annotations := range map[string]map[string]string{
		"no annotation":     nil,
		"annotation false":  {"loft.sh/paused": "false"},
		"other annotations": {"foo": "bar"},
	} {
		t.Run(name, func(t *testing.T) {
			r := &VClusterReconciler{}
			vCluster := newTestVCluster(annotations)
			if err := r.pauseIfNeeded(context.Background(), vCluster); err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if conditions.IsTrue(vCluster, v1alpha1.PausedCondition) {
				t.Fatalf("expected vcluster not to be marked paused")
			}
		})
	}
}

func TestPauseIfNeededNilClientset(t *testing.T) {
	r := &VClusterReconciler{}
	vCluster := newTestVCluster(map[string]string{"loft.sh/paused": "true"})

	err := r.pauseIfNeeded(context.Background(), vCluster)
	if err == nil {
		t.Fatalf("expected an error for a missing clientset, got nil")
	}
	if conditions.IsTrue(vCluster, v1alpha1.PausedCondition) {
		t.Fatalf("expected vcluster not to be marked paused")
	}
}

func TestResumeIfNeededNilClientset(t *testing.T) {
	r := &VClusterReconciler{}
	vCluster := newTestVCluster(map[string]string{"loft.sh/paused": "false"})
	conditions.MarkTrue(vCluster, v1alpha1.PausedCondition)

	if err := r.resumeIfNeeded(context.Background(), vCluster); err == nil {
		t.Fatalf("expected an error for a missing clientset, got nil")
	}
	if !conditions.IsTrue(vCluster, v1alpha1.PausedCondition) {
		t.Fatalf("expected vcluster to stay marked paused")
	}
}

func TestResumeIfNeededNotPaused(t *testing.T) {
	r := &VClusterReconciler{}
	// annotation false but never paused: resume must be a no-op even without a clientset
	vCluster := newTestVCluster(map[string]string{"loft.sh/paused": "false"})

	if err := r.resumeIfNeeded(context.Background(), vCluster); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestPauseIfNeededScalesDownStatefulSet(t *testing.T) {
	api := &fakeAPIServer{}
	srv := httptest.NewServer(api)
	defer srv.Close()

	clientSet, err := kubernetes.NewForConfig(&rest.Config{Host: srv.URL})
	if err != nil {
		t.Fatalf("create clientset: %v", err)
	}

	r := &VClusterReconciler{Clientset: clientSet}
	vCluster := newTestVCluster(map[string]string{"loft.sh/paused": "true"})

	if err := r.pauseIfNeeded(context.Background(), vCluster); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !conditions.IsTrue(vCluster, v1alpha1.PausedCondition) {
		t.Fatalf("expected vcluster to be marked paused")
	}

	api.mu.Lock()
	defer api.mu.Unlock()
	if api.patch == nil {
		t.Fatalf("expected the vcluster statefulset to be patched")
	}
	var patched appsv1.StatefulSet
	if err := json.Unmarshal(api.patch, &patched); err != nil {
		t.Fatalf("decode patch: %v", err)
	}
	if patched.Spec.Replicas == nil || *patched.Spec.Replicas != 0 {
		t.Fatalf("expected statefulset to be scaled to 0, patch was %s", api.patch)
	}
	if patched.Annotations["loft.sh/paused"] != "true" {
		t.Fatalf("expected statefulset to be annotated paused, patch was %s", api.patch)
	}
}

func TestReconcilePersistsPauseStatus(t *testing.T) {
	for _, cleanupError := range []bool{false, true} {
		name := "success"
		if cleanupError {
			name = "cleanup failure"
		}
		t.Run(name, func(t *testing.T) {
			api := &fakeAPIServer{cleanupError: cleanupError}
			srv := httptest.NewServer(api)
			defer srv.Close()
			clientSet, err := kubernetes.NewForConfig(&rest.Config{Host: srv.URL})
			if err != nil {
				t.Fatal(err)
			}
			scheme := runtime.NewScheme()
			if err := v1alpha1.AddToScheme(scheme); err != nil {
				t.Fatal(err)
			}
			vCluster := newTestVCluster(map[string]string{"loft.sh/paused": "true"})
			vCluster.Finalizers = []string{CleanupFinalizer}
			c := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(vCluster).WithObjects(vCluster).Build()
			r := &VClusterReconciler{Client: c, Clientset: clientSet}
			key := types.NamespacedName{Name: testName, Namespace: testNamespace}
			_, err = r.Reconcile(context.Background(), ctrl.Request{NamespacedName: key})
			if (err != nil) != cleanupError {
				t.Fatalf("reconcile error = %v, want error %v", err, cleanupError)
			}
			saved := &v1alpha1.VCluster{}
			if err := c.Get(context.Background(), key, saved); err != nil {
				t.Fatal(err)
			}
			condition := conditions.Get(saved, v1alpha1.PausedCondition)
			if condition == nil {
				t.Fatal("pause condition was not persisted")
			}
			expected := corev1.ConditionTrue
			if cleanupError {
				expected = corev1.ConditionFalse
			}
			if condition.Status != expected {
				t.Fatalf("persisted pause = %s, want %s", condition.Status, expected)
			}
		})
	}
}

func TestPauseSingleNamespaceWorkloads(t *testing.T) {
	for _, mode := range []string{"success", "list failure", "delete failure", "wait failure"} {
		t.Run(mode, func(t *testing.T) {
			api := &fakeAPIServer{
				podListError: mode == "list failure", podDeleteError: mode == "delete failure",
				pods: []corev1.Pod{
					{ObjectMeta: metav1.ObjectMeta{Name: "workload", Labels: map[string]string{"vcluster.loft.sh/managed-by": testName}}},
					{ObjectMeta: metav1.ObjectMeta{Name: "management", Labels: map[string]string{"app": "palette-controller-manager"}}},
					{ObjectMeta: metav1.ObjectMeta{Name: "other-vcluster", Labels: map[string]string{"vcluster.loft.sh/managed-by": "another-vcluster"}}},
				},
			}
			if mode == "wait failure" {
				api.listErrorOn = 2
			}
			srv := httptest.NewServer(api)
			defer srv.Close()
			cs, err := kubernetes.NewForConfig(&rest.Config{Host: srv.URL})
			if err != nil {
				t.Fatal(err)
			}
			r := &VClusterReconciler{Clientset: cs}
			vc := newTestVCluster(map[string]string{"loft.sh/paused": "true"})
			err = r.pauseIfNeeded(context.Background(), vc)
			if mode != "success" {
				if err == nil || conditions.IsTrue(vc, v1alpha1.PausedCondition) {
					t.Fatalf("cleanup failure must not mark paused: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			// Repeated pause must be safe and leave management and unrelated pods intact.
			if err := r.pauseIfNeeded(context.Background(), vc); err != nil {
				t.Fatal(err)
			}
			api.mu.Lock()
			defer api.mu.Unlock()
			if len(api.deleted) != 1 || api.deleted[0] != "workload" {
				t.Fatalf("unexpected deleted pods: %v", api.deleted)
			}
			if len(api.pods) != 2 {
				t.Fatalf("expected management and unrelated pods retained, got %v", api.pods)
			}
			if !conditions.IsTrue(vc, v1alpha1.PausedCondition) {
				t.Fatal("expected paused condition")
			}
		})
	}
}
