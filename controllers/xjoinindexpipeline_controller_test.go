package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/RedHatInsights/strimzi-client-go/apis/kafka.strimzi.io/v1beta2"
	"github.com/jarcoal/httpmock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	v1 "k8s.io/api/apps/v1"
	v12 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	//+kubebuilder:scaffold:imports
)

var _ = Describe("XJoinIndexPipeline", func() {
	var namespace string

	BeforeEach(func() {
		httpmock.Activate()
		httpmock.RegisterNoResponder(httpmock.InitialTransport.RoundTrip) //disable mocks for unregistered http requests

		var err error
		namespace, err = NewNamespace()
		checkError(err)
	})

	AfterEach(func() {
		httpmock.DeactivateAndReset()
	})

	Context("Reconcile Creation", func() {
		It("Should add a finalizer to the indexPipeline", func() {
			reconciler := XJoinIndexPipelineTestReconciler{
				Namespace:      namespace,
				Name:           "test-index-pipeline",
				ConfigFileName: "xjoinindex",
				K8sClient:      k8sClient,
			}
			createdIndexPipeline := reconciler.ReconcileNew()
			Expect(createdIndexPipeline.Finalizers).To(HaveLen(1))
			Expect(createdIndexPipeline.Finalizers).To(ContainElement("finalizer.xjoin.indexpipeline.cloud.redhat.com"))
		})

		It("Should create an Elasticsearch Index", func() {
			reconciler := XJoinIndexPipelineTestReconciler{
				Namespace:      namespace,
				Name:           "test-index-pipeline",
				ConfigFileName: "xjoinindex",
				K8sClient:      k8sClient,
			}
			reconciler.ReconcileNew()

			info := httpmock.GetCallCountInfo()
			count := info["PUT http://localhost:9200/xjoinindexpipeline.test-index-pipeline.1234"]
			Expect(count).To(Equal(1))
		})

		It("Should create an Elasticsearch Connector", func() {
			reconciler := XJoinIndexPipelineTestReconciler{
				Namespace:      namespace,
				Name:           "test-index-pipeline",
				ConfigFileName: "xjoinindex",
				K8sClient:      k8sClient,
			}
			reconciler.ReconcileNew()

			connectorName := "xjoinindexpipeline.test-index-pipeline.1234"
			connectorLookupKey := types.NamespacedName{Name: connectorName, Namespace: namespace}
			elasticsearchConnector := &v1beta2.KafkaConnector{}

			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), connectorLookupKey, elasticsearchConnector)
				if err != nil {
					return false
				}
				return true
			}, K8sGetTimeout, K8sGetInterval).Should(BeTrue())

			elasticsearchClass := "io.confluent.connect.elasticsearch.ElasticsearchSinkConnector"
			connectorPause := false

			Expect(elasticsearchConnector.Name).To(Equal(connectorName))
			Expect(elasticsearchConnector.Namespace).To(Equal(namespace))
			Expect(elasticsearchConnector.Spec.Class).To(Equal(&elasticsearchClass))
			Expect(elasticsearchConnector.Spec.Pause).To(Equal(&connectorPause))
			Expect(elasticsearchConnector.GetLabels()).To(Equal(map[string]string{"strimzi.io/cluster": "connect"}))

			//config comparison
			expectedElasticsearchConfig := LoadExpectedKafkaResourceConfig("./test/data/kafka/elasticsearch_config.json")
			actualElasticsearchConfig := bytes.NewBuffer([]byte{})
			err := json.Compact(actualElasticsearchConfig, elasticsearchConnector.Spec.Config.Raw)
			checkError(err)
			Expect(actualElasticsearchConfig).To(Equal(expectedElasticsearchConfig))
		})

		It("Should create a Kafka Topic", func() {
			reconciler := XJoinIndexPipelineTestReconciler{
				Namespace:      namespace,
				Name:           "test-index-pipeline",
				ConfigFileName: "xjoinindex",
				K8sClient:      k8sClient,
			}
			reconciler.ReconcileNew()

			topicName := "xjoinindexpipeline.test-index-pipeline.1234"
			topicLookupKey := types.NamespacedName{Name: topicName, Namespace: namespace}
			topic := &v1beta2.KafkaTopic{}

			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), topicLookupKey, topic)
				if err != nil {
					return false
				}
				return true
			}, K8sGetTimeout, K8sGetInterval).Should(BeTrue())

			topicReplicas := int32(1)
			topicPartitions := int32(1)

			Expect(topic.Name).To(Equal(topicName))
			Expect(topic.Namespace).To(Equal(namespace))
			Expect(topic.Spec.Replicas).To(Equal(&topicReplicas))
			Expect(topic.Spec.Partitions).To(Equal(&topicPartitions))

			//config comparison
			expectedTopicConfig := LoadExpectedKafkaResourceConfig("./test/data/kafka/topic_config.json")
			actualTopicConfig := bytes.NewBuffer([]byte{})
			err := json.Compact(actualTopicConfig, topic.Spec.Config.Raw)
			checkError(err)
			Expect(actualTopicConfig).To(Equal(expectedTopicConfig))
		})

		It("Should create an Avro Schema", func() {
			reconciler := XJoinIndexPipelineTestReconciler{
				Namespace:      namespace,
				Name:           "test-index-pipeline",
				ConfigFileName: "xjoinindex",
				K8sClient:      k8sClient,
			}
			reconciler.ReconcileNew()

			//TODO validate the body of the request is correct
			//validates the correct API calls were made
			info := httpmock.GetCallCountInfo()
			count := info["GET http://apicurio:1080/apis/ccompat/v6/subjects/xjoinindexpipeline.test-index-pipeline.1234-value/versions/1"]
			Expect(count).To(Equal(1))

			count = info["POST http://apicurio:1080/apis/ccompat/v6/subjects/xjoinindexpipeline.test-index-pipeline.1234-value/versions"]
			Expect(count).To(Equal(1))

			count = info["GET http://apicurio:1080/apis/ccompat/v6/subjects/xjoinindexpipeline.test-index-pipeline.1234-value/versions/latest"]
			Expect(count).To(Equal(1))
		})

		It("Should create a GraphQL Schema", func() {
			reconciler := XJoinIndexPipelineTestReconciler{
				Namespace:            namespace,
				Name:                 "test-index-pipeline",
				ConfigFileName:       "xjoinindex",
				CustomSubgraphImages: nil,
				K8sClient:            k8sClient,
			}
			reconciler.ReconcileNew()

			//TODO validate the body of the request is correct
			//validates the correct API calls were made
			info := httpmock.GetCallCountInfo()
			count := info["GET http://apicurio:1080/apis/registry/v2/groups/default/artifacts/xjoinindexpipeline.test-index-pipeline.1234/versions"]
			Expect(count).To(Equal(1))

			count = info["POST http://apicurio:1080/apis/registry/v2/groups/default/artifacts"]
			Expect(count).To(Equal(1))

			count = info["PUT http://apicurio:1080/apis/registry/v2/groups/default/artifacts/xjoinindexpipeline.test-index-pipeline.1234/meta"]
			Expect(count).To(Equal(1))
		})

		It("Should create an xjoin-core deployment", func() {
			reconciler := XJoinIndexPipelineTestReconciler{
				Namespace:      namespace,
				Name:           "test-index-pipeline",
				ConfigFileName: "xjoinindex",
				K8sClient:      k8sClient,
			}
			reconciler.ReconcileNew()

			deploymentName := "xjoin-core-xjoinindexpipeline-test-index-pipeline-1234"
			deploymentLookupKey := types.NamespacedName{Name: deploymentName, Namespace: namespace}
			deployment := &v1.Deployment{}

			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), deploymentLookupKey, deployment)
				if err != nil {
					return false
				}
				return true
			}, K8sGetTimeout, K8sGetInterval).Should(BeTrue())

			replicas := int32(1)
			revisionHistoryLimit := int32(10)
			progressDeadlineSeconds := int32(600)

			Expect(deployment.Name).To(Equal(deploymentName))
			Expect(deployment.Namespace).To(Equal(namespace))
			Expect(deployment.Spec.Replicas).To(Equal(&replicas))
			Expect(deployment.Spec.Selector.MatchLabels).To(Equal(map[string]string{
				"app":         "xjoin-core-xjoinindexpipeline-test-index-pipeline-1234",
				"xjoin.index": "xjoin-core-xjoinindexpipeline-test-index-pipeline",
			}))
			Expect(deployment.GetLabels()).To(Equal(map[string]string{
				"app":         "xjoin-core-xjoinindexpipeline-test-index-pipeline-1234",
				"xjoin.index": "xjoin-core-xjoinindexpipeline-test-index-pipeline",
			}))
			Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(deployment.Spec.Template.Spec.Containers[0].Name).To(Equal("xjoin-core-xjoinindexpipeline-test-index-pipeline-1234"))
			Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(Equal("quay.io/ckyrouac/xjoin-core:latest"))
			Expect(deployment.Spec.Template.Spec.Containers[0].Env).To(HaveLen(5))
			Expect(deployment.Spec.Template.Spec.Containers[0].Env).To(ContainElements([]v12.EnvVar{
				{
					Name:      "SOURCE_TOPICS",
					Value:     "", //TODO
					ValueFrom: nil,
				},
				{
					Name:      "SINK_TOPIC",
					Value:     "xjoinindexpipeline.test",
					ValueFrom: nil,
				},
				{
					Name:      "SCHEMA_REGISTRY_URL",
					Value:     "http://apicurio:1080/apis/registry/v2",
					ValueFrom: nil,
				},
				{
					Name:      "KAFKA_BOOTSTRAP",
					Value:     "localhost:9092",
					ValueFrom: nil,
				},
				{
					Name:      "SINK_SCHEMA",
					Value:     `{"type":"record","name":"Value","namespace":"test-index-pipeline"}`,
					ValueFrom: nil,
				},
			}))
			Expect(deployment.Spec.Template.Spec.Containers[0].Command).To(BeNil())
			Expect(deployment.Spec.Template.Spec.Containers[0].Args).To(BeNil())
			Expect(deployment.Spec.Template.Spec.Containers[0].Ports).To(BeNil())
			Expect(deployment.Spec.Strategy.Type).To(Equal(v1.DeploymentStrategyType("RollingUpdate")))
			Expect(deployment.Spec.Strategy.RollingUpdate.MaxUnavailable.Type).To(Equal(intstr.Type(1)))
			Expect(deployment.Spec.Strategy.RollingUpdate.MaxUnavailable.IntVal).To(Equal(int32(0)))
			Expect(deployment.Spec.Strategy.RollingUpdate.MaxUnavailable.StrVal).To(Equal("25%"))
			Expect(deployment.Spec.Strategy.RollingUpdate.MaxSurge.Type).To(Equal(intstr.Type(1)))
			Expect(deployment.Spec.Strategy.RollingUpdate.MaxSurge.IntVal).To(Equal(int32(0)))
			Expect(deployment.Spec.Strategy.RollingUpdate.MaxSurge.StrVal).To(Equal("25%"))
			Expect(deployment.Spec.MinReadySeconds).To(Equal(int32(0)))
			Expect(deployment.Spec.RevisionHistoryLimit).To(Equal(&revisionHistoryLimit))
			Expect(deployment.Spec.Paused).To(Equal(false))
			Expect(deployment.Spec.ProgressDeadlineSeconds).To(Equal(&progressDeadlineSeconds))
		})

		It("Should create an xjoin-api-subgraph deployment", func() {
			reconciler := XJoinIndexPipelineTestReconciler{
				Namespace:      namespace,
				Name:           "test-index-pipeline",
				ConfigFileName: "xjoinindex",
				K8sClient:      k8sClient,
			}
			reconciler.ReconcileNew()

			deploymentName := "xjoinindexpipeline-test-index-pipeline-1234"
			deploymentLookupKey := types.NamespacedName{Name: deploymentName, Namespace: namespace}
			deployment := &v1.Deployment{}

			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), deploymentLookupKey, deployment)
				if err != nil {
					return false
				}
				return true
			}, K8sGetTimeout, K8sGetInterval).Should(BeTrue())

			replicas := int32(1)
			revisionHistoryLimit := int32(10)
			progressDeadlineSeconds := int32(600)

			Expect(deployment.Name).To(Equal(deploymentName))
			Expect(deployment.Namespace).To(Equal(namespace))
			Expect(deployment.Spec.Replicas).To(Equal(&replicas))
			Expect(deployment.GetLabels()).To(Equal(map[string]string{
				"app":         "xjoinindexpipeline-test-index-pipeline-1234",
				"xjoin.index": "xjoinindexpipeline-test-index-pipeline",
			}))
			Expect(deployment.Spec.Selector.MatchLabels).To(Equal(map[string]string{
				"app":         "xjoinindexpipeline-test-index-pipeline-1234",
				"xjoin.index": "xjoinindexpipeline-test-index-pipeline",
			}))

			Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(deployment.Spec.Template.Spec.Containers[0].Name).To(Equal("xjoinindexpipeline-test-index-pipeline-1234"))
			Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(Equal("quay.io/ckyrouac/xjoin-api-subgraph:latest"))
			Expect(deployment.Spec.Template.Spec.Containers[0].Env).To(HaveLen(9))
			Expect(deployment.Spec.Template.Spec.Containers[0].Env).To(ContainElements([]v12.EnvVar{
				{
					Name:      "AVRO_SCHEMA",
					Value:     `{"type":"record","name":"Value","namespace":"test-index-pipeline"}`,
					ValueFrom: nil,
				},
				{
					Name:      "SCHEMA_REGISTRY_PROTOCOL",
					Value:     "http",
					ValueFrom: nil,
				},
				{
					Name:      "SCHEMA_REGISTRY_HOSTNAME",
					Value:     "apicurio.test.svc",
					ValueFrom: nil,
				},
				{
					Name:      "SCHEMA_REGISTRY_PORT",
					Value:     "1080",
					ValueFrom: nil,
				},
				{
					Name:      "ELASTIC_SEARCH_URL",
					Value:     "http://localhost:9200",
					ValueFrom: nil,
				},
				{
					Name:      "ELASTIC_SEARCH_USERNAME",
					Value:     "xjoin",
					ValueFrom: nil,
				},
				{
					Name:      "ELASTIC_SEARCH_PASSWORD",
					Value:     "xjoin1337",
					ValueFrom: nil,
				},
				{
					Name:      "ELASTIC_SEARCH_INDEX",
					Value:     "xjoinindexpipeline.test-index-pipeline.1234",
					ValueFrom: nil,
				},
				{
					Name:      "GRAPHQL_SCHEMA_NAME",
					Value:     "xjoinindexpipeline.test-index-pipeline.1234",
					ValueFrom: nil,
				},
			}))
			Expect(deployment.Spec.Template.Spec.Containers[0].Command).To(BeNil())
			Expect(deployment.Spec.Template.Spec.Containers[0].Args).To(BeNil())
			Expect(deployment.Spec.Template.Spec.Containers[0].Ports).To(Equal([]v12.ContainerPort{{
				Name:          "web",
				HostPort:      int32(0),
				ContainerPort: int32(8000),
				Protocol:      "TCP",
				HostIP:        "",
			}}))
			Expect(deployment.Spec.Strategy.Type).To(Equal(v1.DeploymentStrategyType("RollingUpdate")))
			Expect(deployment.Spec.Strategy.RollingUpdate.MaxUnavailable.Type).To(Equal(intstr.Type(1)))
			Expect(deployment.Spec.Strategy.RollingUpdate.MaxUnavailable.IntVal).To(Equal(int32(0)))
			Expect(deployment.Spec.Strategy.RollingUpdate.MaxUnavailable.StrVal).To(Equal("25%"))
			Expect(deployment.Spec.Strategy.RollingUpdate.MaxSurge.Type).To(Equal(intstr.Type(1)))
			Expect(deployment.Spec.Strategy.RollingUpdate.MaxSurge.IntVal).To(Equal(int32(0)))
			Expect(deployment.Spec.Strategy.RollingUpdate.MaxSurge.StrVal).To(Equal("25%"))
			Expect(deployment.Spec.MinReadySeconds).To(Equal(int32(0)))
			Expect(deployment.Spec.RevisionHistoryLimit).To(Equal(&revisionHistoryLimit))
			Expect(deployment.Spec.Paused).To(Equal(false))
			Expect(deployment.Spec.ProgressDeadlineSeconds).To(Equal(&progressDeadlineSeconds))
		})

		It("Should create custom subgraph graphql schema", func() {
			reconciler := XJoinIndexPipelineTestReconciler{
				Namespace:      namespace,
				Name:           "test-index-pipeline",
				ConfigFileName: "xjoinindex",
				K8sClient:      k8sClient,
				CustomSubgraphImages: []v1alpha1.CustomSubgraphImage{{
					Name:  "test-custom-image",
					Image: "quay.io/ckyrouac/host-inventory-subgraph:latest",
				}},
			}
			reconciler.ReconcileNew()

			//TODO validate the body of the request is correct
			//validates the correct API calls were made
			info := httpmock.GetCallCountInfo()
			count := info["GET http://apicurio:1080/apis/registry/v2/groups/default/artifacts/xjoinindexpipeline.test-index-pipeline-test-custom-image.1234/versions"]
			Expect(count).To(Equal(1))

			count = info["POST http://apicurio:1080/apis/registry/v2/groups/default/artifacts"]
			Expect(count).To(Equal(2)) //called once for generic gql schema, then a second time for custom subgraph schema

			count = info["PUT http://apicurio:1080/apis/registry/v2/groups/default/artifacts/xjoinindexpipeline.test-index-pipeline-test-custom-image.1234/meta"]
			Expect(count).To(Equal(1))
		})

		It("Should create custom subgraph deployments", func() {
			reconciler := XJoinIndexPipelineTestReconciler{
				Namespace:      namespace,
				Name:           "test-index-pipeline",
				ConfigFileName: "xjoinindex",
				K8sClient:      k8sClient,
				CustomSubgraphImages: []v1alpha1.CustomSubgraphImage{{
					Name:  "test-custom-image",
					Image: "quay.io/ckyrouac/host-inventory-subgraph:latest",
				}},
			}
			reconciler.ReconcileNew()

			deploymentName := "xjoinindexpipeline-test-index-pipeline-test-custom-image-1234"
			deploymentLookupKey := types.NamespacedName{Name: deploymentName, Namespace: namespace}
			deployment := &v1.Deployment{}

			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), deploymentLookupKey, deployment)
				if err != nil {
					return false
				}
				return true
			}, K8sGetTimeout, K8sGetInterval).Should(BeTrue())

			replicas := int32(1)
			revisionHistoryLimit := int32(10)
			progressDeadlineSeconds := int32(600)

			Expect(deployment.Name).To(Equal(deploymentName))
			Expect(deployment.Namespace).To(Equal(namespace))
			Expect(deployment.Spec.Replicas).To(Equal(&replicas))
			Expect(deployment.GetLabels()).To(Equal(map[string]string{
				"app":         "xjoinindexpipeline-test-index-pipeline-test-custom-image-1234",
				"xjoin.index": "xjoinindexpipeline-test-index-pipeline-test-custom-image",
			}))
			Expect(deployment.Spec.Selector.MatchLabels).To(Equal(map[string]string{
				"app":         "xjoinindexpipeline-test-index-pipeline-test-custom-image-1234",
				"xjoin.index": "xjoinindexpipeline-test-index-pipeline-test-custom-image",
			}))

			Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(deployment.Spec.Template.Spec.Containers[0].Name).To(Equal(deploymentName))
			Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(Equal("quay.io/ckyrouac/host-inventory-subgraph:latest"))
			Expect(deployment.Spec.Template.Spec.Containers[0].Env).To(HaveLen(9))
			Expect(deployment.Spec.Template.Spec.Containers[0].Env).To(ContainElements([]v12.EnvVar{
				{
					Name:      "AVRO_SCHEMA",
					Value:     `{"type":"record","name":"Value","namespace":"test-index-pipeline"}`,
					ValueFrom: nil,
				},
				{
					Name:      "SCHEMA_REGISTRY_PROTOCOL",
					Value:     "http",
					ValueFrom: nil,
				},
				{
					Name:      "SCHEMA_REGISTRY_HOSTNAME",
					Value:     "apicurio.test.svc",
					ValueFrom: nil,
				},
				{
					Name:      "SCHEMA_REGISTRY_PORT",
					Value:     "1080",
					ValueFrom: nil,
				},
				{
					Name:      "ELASTIC_SEARCH_URL",
					Value:     "http://localhost:9200",
					ValueFrom: nil,
				},
				{
					Name:      "ELASTIC_SEARCH_USERNAME",
					Value:     "xjoin",
					ValueFrom: nil,
				},
				{
					Name:      "ELASTIC_SEARCH_PASSWORD",
					Value:     "xjoin1337",
					ValueFrom: nil,
				},
				{
					Name:      "ELASTIC_SEARCH_INDEX",
					Value:     "xjoinindexpipeline.test-index-pipeline.1234",
					ValueFrom: nil,
				},
				{
					Name:      "GRAPHQL_SCHEMA_NAME",
					Value:     "xjoinindexpipeline.test-index-pipeline-test-custom-image.1234",
					ValueFrom: nil,
				},
			}))
			Expect(deployment.Spec.Template.Spec.Containers[0].Command).To(BeNil())
			Expect(deployment.Spec.Template.Spec.Containers[0].Args).To(BeNil())
			Expect(deployment.Spec.Template.Spec.Containers[0].Ports).To(Equal([]v12.ContainerPort{{
				Name:          "web",
				HostPort:      int32(0),
				ContainerPort: int32(8000),
				Protocol:      "TCP",
				HostIP:        "",
			}}))
			Expect(deployment.Spec.Strategy.Type).To(Equal(v1.DeploymentStrategyType("RollingUpdate")))
			Expect(deployment.Spec.Strategy.RollingUpdate.MaxUnavailable.Type).To(Equal(intstr.Type(1)))
			Expect(deployment.Spec.Strategy.RollingUpdate.MaxUnavailable.IntVal).To(Equal(int32(0)))
			Expect(deployment.Spec.Strategy.RollingUpdate.MaxUnavailable.StrVal).To(Equal("25%"))
			Expect(deployment.Spec.Strategy.RollingUpdate.MaxSurge.Type).To(Equal(intstr.Type(1)))
			Expect(deployment.Spec.Strategy.RollingUpdate.MaxSurge.IntVal).To(Equal(int32(0)))
			Expect(deployment.Spec.Strategy.RollingUpdate.MaxSurge.StrVal).To(Equal("25%"))
			Expect(deployment.Spec.MinReadySeconds).To(Equal(int32(0)))
			Expect(deployment.Spec.RevisionHistoryLimit).To(Equal(&revisionHistoryLimit))
			Expect(deployment.Spec.Paused).To(Equal(false))
			Expect(deployment.Spec.ProgressDeadlineSeconds).To(Equal(&progressDeadlineSeconds))
		})

		It("Should create an Elasticsearch Pipeline when the AvroSchema contains at least one JSON field", func() {
			dataSourceName := "testdatasource"
			datasourceReconciler := DatasourceTestReconciler{
				Namespace: namespace,
				Name:      dataSourceName,
				K8sClient: k8sClient,
			}
			datasourceReconciler.ReconcileNew()
			createdDataSource := datasourceReconciler.ReconcileValid()

			reconciler := XJoinIndexPipelineTestReconciler{
				Namespace:      namespace,
				Name:           "test-index-pipeline",
				ConfigFileName: "xjoinindex-with-json-field",
				K8sClient:      k8sClient,
				DataSources: []DataSource{{
					Name:                     dataSourceName,
					Version:                  createdDataSource.Status.ActiveVersion,
					ApiCurioResponseFilename: "datasource-latest-version",
				}},
			}
			reconciler.ReconcileNew()

			info := httpmock.GetCallCountInfo()
			count := info["GET http://apicurio:1080/apis/ccompat/v6/subjects/xjoindatasourcepipeline.testdatasource."+createdDataSource.Status.ActiveVersion+"-value/versions/latest"]
			Expect(count).To(Equal(1))

			count = info["GET http://localhost:9200/_ingest/pipeline/xjoinindexpipeline.test-index-pipeline.1234"]
			Expect(count).To(Equal(1))

			count = info["PUT http://localhost:9200/_ingest/pipeline/xjoinindexpipeline.test-index-pipeline.1234"]
			Expect(count).To(Equal(1))
		})
	})

	Context("Reconcile Deletion", func() {
		It("Should delete the Elasticsearch index", func() {
			Skip("WIP")
			reconciler := XJoinIndexPipelineTestReconciler{
				Namespace:      namespace,
				Name:           "test-index-pipeline",
				ConfigFileName: "xjoinindex",
				K8sClient:      k8sClient,
			}
			createdIndexPipeline := reconciler.ReconcileNew()
			err := k8sClient.Delete(context.Background(), &createdIndexPipeline)
			checkError(err)
			reconciler.ReconcileDelete()
		})

		It("Should delete the Elasticsearch connector", func() {

		})

		It("Should delete the Kafka topic", func() {

		})

		It("Should delete the Avro schema", func() {

		})

		It("Should delete the GraphQL schema", func() {

		})

		It("Should delete the xjoin-core deployment", func() {

		})

		It("Should delete the xjoin-api-subgraph deployment", func() {

		})

		It("Should delete the custom image's xjoin-api-subgraph deployment", func() {

		})

		It("Should delete the custom image's graphql schema", func() {

		})

		It("Should delete the Elasticsearch pipeline", func() {

		})
	})
})