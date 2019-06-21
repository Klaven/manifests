package tests_test

import (
	"sigs.k8s.io/kustomize/k8sdeps/kunstruct"
	"sigs.k8s.io/kustomize/k8sdeps/transformer"
	"sigs.k8s.io/kustomize/pkg/fs"
	"sigs.k8s.io/kustomize/pkg/loader"
	"sigs.k8s.io/kustomize/pkg/resmap"
	"sigs.k8s.io/kustomize/pkg/resource"
	"sigs.k8s.io/kustomize/pkg/target"
	"testing"
)

func writeMetadataBase(th *KustTestHarness) {
	th.writeF("/manifests/metadata/base/metadata-db-pvc.yaml", `
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: mysql
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
`)
	th.writeF("/manifests/metadata/base/metadata-db-secret.yaml", `
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: db-secrets
data:
  MYSQL_ROOT_PASSWORD: dGVzdA== # "test"
`)
	th.writeF("/manifests/metadata/base/metadata-db-deployment.yaml", `
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: db
  labels:
    component: db
spec:
  replicas: 1
  template:
    metadata:
      name: db
      labels:
        component: db
    spec:
      containers:
      - name: db-container
        image: mysql:8.0.3
        args:
        - --datadir
        - /var/lib/mysql/datadir
        env:
          - name: MYSQL_ROOT_PASSWORD
            valueFrom:
              secretKeyRef:
                name: metadata-db-secrets
                key: MYSQL_ROOT_PASSWORD
          - name: MYSQL_ALLOW_EMPTY_PASSWORD
            value: "true"
          - name: MYSQL_DATABASE
            value: "metadb"
        ports:
        - name: dbapi
          containerPort: 3306
        readinessProbe:
          exec:
            command:
            - "/bin/bash"
            - "-c"
            - "mysql -D $$MYSQL_DATABASE -p$$MYSQL_ROOT_PASSWORD -e 'SELECT 1'"
          initialDelaySeconds: 5
          periodSeconds: 2
          timeoutSeconds: 1
        volumeMounts:
        - name: metadata-mysql
          mountPath: /var/lib/mysql
      volumes:
      - name: metadata-mysql
        persistentVolumeClaim:
          claimName: metadata-mysql
`)
	th.writeF("/manifests/metadata/base/metadata-db-service.yaml", `
apiVersion: v1
kind: Service
metadata:
  name: db
  labels:
    component: db
spec:
  type: ClusterIP
  ports:
    - port: 3306
      protocol: TCP
      name: dbapi
  selector:
    component: db
`)
	th.writeF("/manifests/metadata/base/metadata-deployment.yaml", `
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: deployment
  labels:
    component: server
spec:
  replicas: 3
  template:
    metadata:
      labels:
        component: server
    spec:
      containers:
      - name: container
        image: gcr.io/kubeflow-images-public/metadata:v0.1.4
        env:
          - name: MYSQL_ROOT_PASSWORD
            valueFrom:
              secretKeyRef:
                name: metadata-db-secrets
                key: MYSQL_ROOT_PASSWORD
        command: ["./server/server",
                  "--http_port=8080",
                  "--mysql_service_host=metadata-db.kubeflow",
                  "--mysql_service_port=3306",
                  "--mysql_service_user=root",
                  "--mysql_service_password=$(MYSQL_ROOT_PASSWORD)",
                  "--mlmd_db_name=metadb"]
        ports:
        - containerPort: 8080
`)
	th.writeF("/manifests/metadata/base/metadata-service.yaml", `
kind: Service
apiVersion: v1
metadata:
  labels:
    app: metadata
  name: service
spec:
  selector:
    app: metadata
  type: LoadBalancer
  ports:
  - protocol: TCP
    port: 8666
    targetPort: 8080
`)
	th.writeK("/manifests/metadata/base", `
namePrefix: metadata-

apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
commonLabels:
  kustomize.component: metadata
resources:
- metadata-db-pvc.yaml
- metadata-db-secret.yaml
- metadata-db-deployment.yaml
- metadata-db-service.yaml
- metadata-deployment.yaml
- metadata-service.yaml

`)
}

func TestMetadataBase(t *testing.T) {
	th := NewKustTestHarness(t, "/manifests/metadata/base")
	writeMetadataBase(th)
	m, err := th.makeKustTarget().MakeCustomizedResMap()
	if err != nil {
		t.Fatalf("Err: %v", err)
	}
	targetPath := "../metadata/base"
	fsys := fs.MakeRealFS()
	_loader, loaderErr := loader.NewLoader(targetPath, fsys)
	if loaderErr != nil {
		t.Fatalf("could not load kustomize loader: %v", loaderErr)
	}
	rf := resmap.NewFactory(resource.NewFactory(kunstruct.NewKunstructuredFactoryImpl()))
	kt, err := target.NewKustTarget(_loader, rf, transformer.NewFactoryImpl())
	if err != nil {
		th.t.Fatalf("Unexpected construction error %v", err)
	}
	n, err := kt.MakeCustomizedResMap()
	if err != nil {
		t.Fatalf("Err: %v", err)
	}
	expected, err := n.EncodeAsYaml()
	th.assertActualEqualsExpected(m, string(expected))
}
