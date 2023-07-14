package tests

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fluxcd/pkg/runtime/testenv"
	templatesv1 "github.com/weaveworks/flux-shard-controller/api/v1alpha1"
	"github.com/weaveworks/flux-shard-controller/test"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/weaveworks/flux-shard-controller/internal/controller"
)

const (
	timeout = 10 * time.Second
)

var (
	testEnv *testenv.Environment
	ctx     = ctrl.SetupSignalHandler()
)

func TestMain(m *testing.M) {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme.Scheme))
	utilruntime.Must(templatesv1.AddToScheme(scheme.Scheme))
	testEnv = testenv.New(testenv.WithCRDPath(
		filepath.Join("..", "..", "config", "crd", "bases"),
	))

	if err := testEnv.Create(context.TODO(), test.NewNamespace("flux-system")); err != nil {
		panic(fmt.Sprintf("failed to create namespace flux-system: %s", err))
	}

	if err := (&controller.FluxShardSetReconciler{
		Client: testEnv,
		Scheme: testEnv.GetScheme(),
	}).SetupWithManager(testEnv); err != nil {
		panic(fmt.Sprintf("Failed to start FluxShardSetReconciler: %v", err))
	}

	go func() {
		fmt.Println("Starting the test environment")
		if err := testEnv.Start(ctx); err != nil {
			panic(fmt.Sprintf("Failed to start the test environment manager: %v", err))
		}
	}()
	<-testEnv.Manager.Elected()

	code := m.Run()

	fmt.Println("Stopping the test environment")
	if err := testEnv.Stop(); err != nil {
		panic(fmt.Sprintf("Failed to stop the test environment: %v", err))
	}

	os.Exit(code)
}
