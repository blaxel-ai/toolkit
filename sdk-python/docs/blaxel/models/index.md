Module beamlit.models
=====================
Contains all the data models used in inputs/outputs

Sub-modules
-----------
* beamlit.models.acl
* beamlit.models.agent
* beamlit.models.agent_chain
* beamlit.models.agent_spec
* beamlit.models.api_key
* beamlit.models.configuration
* beamlit.models.continent
* beamlit.models.core_event
* beamlit.models.core_spec
* beamlit.models.core_spec_configurations
* beamlit.models.country
* beamlit.models.create_api_key_for_service_account_body
* beamlit.models.create_workspace_service_account_body
* beamlit.models.create_workspace_service_account_response_200
* beamlit.models.delete_workspace_service_account_response_200
* beamlit.models.entrypoint
* beamlit.models.entrypoint_env
* beamlit.models.flavor
* beamlit.models.form
* beamlit.models.form_config
* beamlit.models.form_oauthomitempty
* beamlit.models.form_secrets
* beamlit.models.function
* beamlit.models.function_kit
* beamlit.models.function_spec
* beamlit.models.get_trace_ids_response_200
* beamlit.models.get_trace_logs_response_200
* beamlit.models.get_trace_response_200
* beamlit.models.get_workspace_service_accounts_response_200_item
* beamlit.models.histogram_bucket
* beamlit.models.histogram_stats
* beamlit.models.integration_connection
* beamlit.models.integration_connection_spec
* beamlit.models.integration_connection_spec_config
* beamlit.models.integration_connection_spec_secret
* beamlit.models.integration_model
* beamlit.models.integration_repository
* beamlit.models.invite_workspace_user_body
* beamlit.models.knowledgebase
* beamlit.models.knowledgebase_spec
* beamlit.models.knowledgebase_spec_options
* beamlit.models.last_n_requests_metric
* beamlit.models.latency_metric
* beamlit.models.location_response
* beamlit.models.mcp_definition
* beamlit.models.mcp_definition_entrypoint
* beamlit.models.mcp_definition_form
* beamlit.models.metadata
* beamlit.models.metadata_labels
* beamlit.models.metric
* beamlit.models.metrics
* beamlit.models.metrics_models
* beamlit.models.metrics_request_total_per_code
* beamlit.models.metrics_rps_per_code
* beamlit.models.model
* beamlit.models.model_private_cluster
* beamlit.models.model_spec
* beamlit.models.o_auth
* beamlit.models.owner_fields
* beamlit.models.pending_invitation
* beamlit.models.pending_invitation_accept
* beamlit.models.pending_invitation_render
* beamlit.models.pending_invitation_render_invited_by
* beamlit.models.pending_invitation_render_workspace
* beamlit.models.pending_invitation_workspace_details
* beamlit.models.pod_template_spec
* beamlit.models.policy
* beamlit.models.policy_location
* beamlit.models.policy_max_tokens
* beamlit.models.policy_spec
* beamlit.models.private_cluster
* beamlit.models.private_location
* beamlit.models.repository
* beamlit.models.request_duration_over_time_metric
* beamlit.models.request_duration_over_time_metrics
* beamlit.models.request_total_by_origin_metric
* beamlit.models.request_total_by_origin_metric_request_total_by_origin
* beamlit.models.request_total_by_origin_metric_request_total_by_origin_and_code
* beamlit.models.request_total_metric
* beamlit.models.request_total_metric_request_total_per_code
* beamlit.models.request_total_metric_rps_per_code
* beamlit.models.resource_log
* beamlit.models.resource_metrics
* beamlit.models.resource_metrics_request_total_per_code
* beamlit.models.resource_metrics_rps_per_code
* beamlit.models.revision_metadata
* beamlit.models.runtime
* beamlit.models.runtime_readiness_probe
* beamlit.models.runtime_resources
* beamlit.models.serverless_config
* beamlit.models.spec_configuration
* beamlit.models.store_agent
* beamlit.models.store_agent_labels
* beamlit.models.store_configuration
* beamlit.models.store_configuration_option
* beamlit.models.store_function
* beamlit.models.store_function_kit
* beamlit.models.store_function_labels
* beamlit.models.store_function_parameter
* beamlit.models.time_fields
* beamlit.models.token_rate_metric
* beamlit.models.token_rate_metrics
* beamlit.models.token_total_metric
* beamlit.models.trace_ids_response
* beamlit.models.update_workspace_service_account_body
* beamlit.models.update_workspace_service_account_response_200
* beamlit.models.update_workspace_user_role_body
* beamlit.models.websocket_channel
* beamlit.models.workspace
* beamlit.models.workspace_labels
* beamlit.models.workspace_user

Classes
-------

`ACL(created_at: beamlit.types.Unset | str = <beamlit.types.Unset object>, updated_at: beamlit.types.Unset | str = <beamlit.types.Unset object>, id: beamlit.types.Unset | str = <beamlit.types.Unset object>, resource_id: beamlit.types.Unset | str = <beamlit.types.Unset object>, resource_type: beamlit.types.Unset | str = <beamlit.types.Unset object>, role: beamlit.types.Unset | str = <beamlit.types.Unset object>, subject_id: beamlit.types.Unset | str = <beamlit.types.Unset object>, subject_type: beamlit.types.Unset | str = <beamlit.types.Unset object>, workspace: beamlit.types.Unset | str = <beamlit.types.Unset object>)`
:   ACL
    
    Attributes:
        created_at (Union[Unset, str]): The date and time when the resource was created
        updated_at (Union[Unset, str]): The date and time when the resource was updated
        id (Union[Unset, str]): ACL id
        resource_id (Union[Unset, str]): Resource ID
        resource_type (Union[Unset, str]): Resource type
        role (Union[Unset, str]): Role
        subject_id (Union[Unset, str]): Subject ID
        subject_type (Union[Unset, str]): Subject type
        workspace (Union[Unset, str]): Workspace name
    
    Method generated by attrs for class ACL.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    `created_at: beamlit.types.Unset | str`
    :

    `id: beamlit.types.Unset | str`
    :

    `resource_id: beamlit.types.Unset | str`
    :

    `resource_type: beamlit.types.Unset | str`
    :

    `role: beamlit.types.Unset | str`
    :

    `subject_id: beamlit.types.Unset | str`
    :

    `subject_type: beamlit.types.Unset | str`
    :

    `updated_at: beamlit.types.Unset | str`
    :

    `workspace: beamlit.types.Unset | str`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`Agent(events: beamlit.types.Unset | list['CoreEvent'] = <beamlit.types.Unset object>, metadata: beamlit.types.Unset | ForwardRef('Metadata') = <beamlit.types.Unset object>, spec: beamlit.types.Unset | ForwardRef('AgentSpec') = <beamlit.types.Unset object>, status: beamlit.types.Unset | str = <beamlit.types.Unset object>)`
:   Agent
    
    Attributes:
        events (Union[Unset, list['CoreEvent']]): Core events
        metadata (Union[Unset, Metadata]): Metadata
        spec (Union[Unset, AgentSpec]): Agent specification
        status (Union[Unset, str]): Agent status
    
    Method generated by attrs for class Agent.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties`
    :

    `events`
    :

    `metadata`
    :

    `spec`
    :

    `status`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`AgentChain(description: beamlit.types.Unset | str = <beamlit.types.Unset object>, enabled: beamlit.types.Unset | bool = <beamlit.types.Unset object>, name: beamlit.types.Unset | str = <beamlit.types.Unset object>, prompt: beamlit.types.Unset | str = <beamlit.types.Unset object>)`
:   Agent chain configuration
    
    Attributes:
        description (Union[Unset, str]): Description of the agent in case you want to override the default one
        enabled (Union[Unset, bool]): Whether the agent chain is enabled
        name (Union[Unset, str]): The name of the agent to chain to
        prompt (Union[Unset, str]): Prompt of the agent in case you want to override the default one
    
    Method generated by attrs for class AgentChain.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    `description: beamlit.types.Unset | str`
    :

    `enabled: beamlit.types.Unset | bool`
    :

    `name: beamlit.types.Unset | str`
    :

    `prompt: beamlit.types.Unset | str`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`AgentSpec(configurations: beamlit.types.Unset | ForwardRef('CoreSpecConfigurations') = <beamlit.types.Unset object>, enabled: beamlit.types.Unset | bool = <beamlit.types.Unset object>, flavors: beamlit.types.Unset | list['Flavor'] = <beamlit.types.Unset object>, integration_connections: beamlit.types.Unset | list[str] = <beamlit.types.Unset object>, pod_template: beamlit.types.Unset | ForwardRef('PodTemplateSpec') = <beamlit.types.Unset object>, policies: beamlit.types.Unset | list[str] = <beamlit.types.Unset object>, private_clusters: beamlit.types.Unset | ForwardRef('ModelPrivateCluster') = <beamlit.types.Unset object>, runtime: beamlit.types.Unset | ForwardRef('Runtime') = <beamlit.types.Unset object>, sandbox: beamlit.types.Unset | bool = <beamlit.types.Unset object>, serverless_config: beamlit.types.Unset | ForwardRef('ServerlessConfig') = <beamlit.types.Unset object>, agent_chain: beamlit.types.Unset | list['AgentChain'] = <beamlit.types.Unset object>, description: beamlit.types.Unset | str = <beamlit.types.Unset object>, functions: beamlit.types.Unset | list[str] = <beamlit.types.Unset object>, knowledgebase: beamlit.types.Unset | str = <beamlit.types.Unset object>, model: beamlit.types.Unset | str = <beamlit.types.Unset object>, prompt: beamlit.types.Unset | str = <beamlit.types.Unset object>, repository: beamlit.types.Unset | ForwardRef('Repository') = <beamlit.types.Unset object>, store_id: beamlit.types.Unset | str = <beamlit.types.Unset object>)`
:   Agent specification
    
    Attributes:
        configurations (Union[Unset, CoreSpecConfigurations]): Optional configurations for the object
        enabled (Union[Unset, bool]): Enable or disable the agent
        flavors (Union[Unset, list['Flavor']]): Types of hardware available for deployments
        integration_connections (Union[Unset, list[str]]):
        pod_template (Union[Unset, PodTemplateSpec]): Pod template specification
        policies (Union[Unset, list[str]]):
        private_clusters (Union[Unset, ModelPrivateCluster]): Private cluster where the model deployment is deployed
        runtime (Union[Unset, Runtime]): Set of configurations for a deployment
        sandbox (Union[Unset, bool]): Sandbox mode
        serverless_config (Union[Unset, ServerlessConfig]): Configuration for a serverless deployment
        agent_chain (Union[Unset, list['AgentChain']]): Agent chain
        description (Union[Unset, str]): Description, small description computed from the prompt
        functions (Union[Unset, list[str]]):
        knowledgebase (Union[Unset, str]): Knowledgebase Name
        model (Union[Unset, str]): Model name
        prompt (Union[Unset, str]): Prompt, describe what your agent does
        repository (Union[Unset, Repository]): Repository
        store_id (Union[Unset, str]): Store id
    
    Method generated by attrs for class AgentSpec.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties`
    :

    `agent_chain`
    :

    `configurations`
    :

    `description`
    :

    `enabled`
    :

    `flavors`
    :

    `functions`
    :

    `integration_connections`
    :

    `knowledgebase`
    :

    `model`
    :

    `pod_template`
    :

    `policies`
    :

    `private_clusters`
    :

    `prompt`
    :

    `repository`
    :

    `runtime`
    :

    `sandbox`
    :

    `serverless_config`
    :

    `store_id`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`ApiKey(created_at: beamlit.types.Unset | str = <beamlit.types.Unset object>, updated_at: beamlit.types.Unset | str = <beamlit.types.Unset object>, created_by: beamlit.types.Unset | str = <beamlit.types.Unset object>, updated_by: beamlit.types.Unset | str = <beamlit.types.Unset object>, api_key: beamlit.types.Unset | str = <beamlit.types.Unset object>, expires_in: beamlit.types.Unset | str = <beamlit.types.Unset object>, id: beamlit.types.Unset | str = <beamlit.types.Unset object>, name: beamlit.types.Unset | str = <beamlit.types.Unset object>, sub: beamlit.types.Unset | str = <beamlit.types.Unset object>, sub_type: beamlit.types.Unset | str = <beamlit.types.Unset object>)`
:   Long-lived API key for accessing Beamlit
    
    Attributes:
        created_at (Union[Unset, str]): The date and time when the resource was created
        updated_at (Union[Unset, str]): The date and time when the resource was updated
        created_by (Union[Unset, str]): The user or service account who created the resource
        updated_by (Union[Unset, str]): The user or service account who updated the resource
        api_key (Union[Unset, str]): Api key
        expires_in (Union[Unset, str]): Duration until expiration (in seconds)
        id (Union[Unset, str]): Api key id, to retrieve it from the API
        name (Union[Unset, str]): Name for the API key
        sub (Union[Unset, str]): User subject identifier
        sub_type (Union[Unset, str]): Subject type
    
    Method generated by attrs for class ApiKey.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    `api_key: beamlit.types.Unset | str`
    :

    `created_at: beamlit.types.Unset | str`
    :

    `created_by: beamlit.types.Unset | str`
    :

    `expires_in: beamlit.types.Unset | str`
    :

    `id: beamlit.types.Unset | str`
    :

    `name: beamlit.types.Unset | str`
    :

    `sub: beamlit.types.Unset | str`
    :

    `sub_type: beamlit.types.Unset | str`
    :

    `updated_at: beamlit.types.Unset | str`
    :

    `updated_by: beamlit.types.Unset | str`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`Configuration(continents: beamlit.types.Unset | list[typing.Any] = <beamlit.types.Unset object>, countries: beamlit.types.Unset | list[typing.Any] = <beamlit.types.Unset object>, private_locations: beamlit.types.Unset | list[typing.Any] = <beamlit.types.Unset object>)`
:   Configuration
    
    Attributes:
        continents (Union[Unset, list[Any]]): Continents
        countries (Union[Unset, list[Any]]): Countries
        private_locations (Union[Unset, list[Any]]): Private locations managed with beamlit operator
    
    Method generated by attrs for class Configuration.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    `continents: beamlit.types.Unset | list[typing.Any]`
    :

    `countries: beamlit.types.Unset | list[typing.Any]`
    :

    `private_locations: beamlit.types.Unset | list[typing.Any]`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`Continent(display_name: beamlit.types.Unset | str = <beamlit.types.Unset object>, name: beamlit.types.Unset | str = <beamlit.types.Unset object>)`
:   Continent
    
    Attributes:
        display_name (Union[Unset, str]): Continent display name
        name (Union[Unset, str]): Continent code
    
    Method generated by attrs for class Continent.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    `display_name: beamlit.types.Unset | str`
    :

    `name: beamlit.types.Unset | str`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`CoreEvent(message: beamlit.types.Unset | str = <beamlit.types.Unset object>, status: beamlit.types.Unset | str = <beamlit.types.Unset object>, time: beamlit.types.Unset | str = <beamlit.types.Unset object>, type_: beamlit.types.Unset | str = <beamlit.types.Unset object>)`
:   Core event
    
    Attributes:
        message (Union[Unset, str]): Event message
        status (Union[Unset, str]): Event status
        time (Union[Unset, str]): Event time
        type_ (Union[Unset, str]): Event type
    
    Method generated by attrs for class CoreEvent.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    `message: beamlit.types.Unset | str`
    :

    `status: beamlit.types.Unset | str`
    :

    `time: beamlit.types.Unset | str`
    :

    `type_: beamlit.types.Unset | str`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`CoreSpec(configurations: beamlit.types.Unset | ForwardRef('CoreSpecConfigurations') = <beamlit.types.Unset object>, enabled: beamlit.types.Unset | bool = <beamlit.types.Unset object>, flavors: beamlit.types.Unset | list['Flavor'] = <beamlit.types.Unset object>, integration_connections: beamlit.types.Unset | list[str] = <beamlit.types.Unset object>, pod_template: beamlit.types.Unset | ForwardRef('PodTemplateSpec') = <beamlit.types.Unset object>, policies: beamlit.types.Unset | list[str] = <beamlit.types.Unset object>, private_clusters: beamlit.types.Unset | ForwardRef('ModelPrivateCluster') = <beamlit.types.Unset object>, runtime: beamlit.types.Unset | ForwardRef('Runtime') = <beamlit.types.Unset object>, sandbox: beamlit.types.Unset | bool = <beamlit.types.Unset object>, serverless_config: beamlit.types.Unset | ForwardRef('ServerlessConfig') = <beamlit.types.Unset object>)`
:   Core specification
    
    Attributes:
        configurations (Union[Unset, CoreSpecConfigurations]): Optional configurations for the object
        enabled (Union[Unset, bool]): Enable or disable the agent
        flavors (Union[Unset, list['Flavor']]): Types of hardware available for deployments
        integration_connections (Union[Unset, list[str]]):
        pod_template (Union[Unset, PodTemplateSpec]): Pod template specification
        policies (Union[Unset, list[str]]):
        private_clusters (Union[Unset, ModelPrivateCluster]): Private cluster where the model deployment is deployed
        runtime (Union[Unset, Runtime]): Set of configurations for a deployment
        sandbox (Union[Unset, bool]): Sandbox mode
        serverless_config (Union[Unset, ServerlessConfig]): Configuration for a serverless deployment
    
    Method generated by attrs for class CoreSpec.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties`
    :

    `configurations`
    :

    `enabled`
    :

    `flavors`
    :

    `integration_connections`
    :

    `pod_template`
    :

    `policies`
    :

    `private_clusters`
    :

    `runtime`
    :

    `sandbox`
    :

    `serverless_config`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`CoreSpecConfigurations(key: beamlit.types.Unset | ForwardRef('SpecConfiguration') = <beamlit.types.Unset object>)`
:   Optional configurations for the object
    
    Attributes:
        key (Union[Unset, SpecConfiguration]): Configuration, this is a key value storage. In your object you can
            retrieve the value with config[key]
    
    Method generated by attrs for class CoreSpecConfigurations.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties`
    :

    `key`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`Country(display_name: beamlit.types.Unset | str = <beamlit.types.Unset object>, name: beamlit.types.Unset | str = <beamlit.types.Unset object>)`
:   Configuration
    
    Attributes:
        display_name (Union[Unset, str]): Country display name
        name (Union[Unset, str]): Country code
    
    Method generated by attrs for class Country.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    `display_name: beamlit.types.Unset | str`
    :

    `name: beamlit.types.Unset | str`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`CreateApiKeyForServiceAccountBody(expires_in: beamlit.types.Unset | str = <beamlit.types.Unset object>, name: beamlit.types.Unset | str = <beamlit.types.Unset object>)`
:   Attributes:
        expires_in (Union[Unset, str]): Expiration period for the API key
        name (Union[Unset, str]): Name for the API key
    
    Method generated by attrs for class CreateApiKeyForServiceAccountBody.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    `expires_in: beamlit.types.Unset | str`
    :

    `name: beamlit.types.Unset | str`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`CreateWorkspaceServiceAccountBody(name: str, description: beamlit.types.Unset | str = <beamlit.types.Unset object>)`
:   Attributes:
        name (str): Service account name
        description (Union[Unset, str]): Service account description
    
    Method generated by attrs for class CreateWorkspaceServiceAccountBody.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    `description: beamlit.types.Unset | str`
    :

    `name: str`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`CreateWorkspaceServiceAccountResponse200(client_id: beamlit.types.Unset | str = <beamlit.types.Unset object>, client_secret: beamlit.types.Unset | str = <beamlit.types.Unset object>, created_at: beamlit.types.Unset | str = <beamlit.types.Unset object>, description: beamlit.types.Unset | str = <beamlit.types.Unset object>, name: beamlit.types.Unset | str = <beamlit.types.Unset object>, updated_at: beamlit.types.Unset | str = <beamlit.types.Unset object>)`
:   Attributes:
        client_id (Union[Unset, str]): Service account client ID
        client_secret (Union[Unset, str]): Service account client secret (only returned on creation)
        created_at (Union[Unset, str]): Creation timestamp
        description (Union[Unset, str]): Service account description
        name (Union[Unset, str]): Service account name
        updated_at (Union[Unset, str]): Last update timestamp
    
    Method generated by attrs for class CreateWorkspaceServiceAccountResponse200.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    `client_id: beamlit.types.Unset | str`
    :

    `client_secret: beamlit.types.Unset | str`
    :

    `created_at: beamlit.types.Unset | str`
    :

    `description: beamlit.types.Unset | str`
    :

    `name: beamlit.types.Unset | str`
    :

    `updated_at: beamlit.types.Unset | str`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`DeleteWorkspaceServiceAccountResponse200(client_id: beamlit.types.Unset | str = <beamlit.types.Unset object>, created_at: beamlit.types.Unset | str = <beamlit.types.Unset object>, description: beamlit.types.Unset | str = <beamlit.types.Unset object>, name: beamlit.types.Unset | str = <beamlit.types.Unset object>, updated_at: beamlit.types.Unset | str = <beamlit.types.Unset object>)`
:   Attributes:
        client_id (Union[Unset, str]): Service account client ID
        created_at (Union[Unset, str]): Creation timestamp
        description (Union[Unset, str]): Service account description
        name (Union[Unset, str]): Service account name
        updated_at (Union[Unset, str]): Last update timestamp
    
    Method generated by attrs for class DeleteWorkspaceServiceAccountResponse200.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    `client_id: beamlit.types.Unset | str`
    :

    `created_at: beamlit.types.Unset | str`
    :

    `description: beamlit.types.Unset | str`
    :

    `name: beamlit.types.Unset | str`
    :

    `updated_at: beamlit.types.Unset | str`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`Entrypoint(args: beamlit.types.Unset | list[typing.Any] = <beamlit.types.Unset object>, command: beamlit.types.Unset | str = <beamlit.types.Unset object>, env: beamlit.types.Unset | ForwardRef('EntrypointEnv') = <beamlit.types.Unset object>)`
:   Entrypoint of the artifact
    
    Attributes:
        args (Union[Unset, list[Any]]): Args of the entrypoint
        command (Union[Unset, str]): Command of the entrypoint
        env (Union[Unset, EntrypointEnv]): Env of the entrypoint
    
    Method generated by attrs for class Entrypoint.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties`
    :

    `args`
    :

    `command`
    :

    `env`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`EntrypointEnv()`
:   Env of the entrypoint
    
    Method generated by attrs for class EntrypointEnv.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`Flavor(name: beamlit.types.Unset | str = <beamlit.types.Unset object>, type_: beamlit.types.Unset | str = <beamlit.types.Unset object>)`
:   A type of hardware available for deployments
    
    Attributes:
        name (Union[Unset, str]): Flavor name (e.g. t4)
        type_ (Union[Unset, str]): Flavor type (e.g. cpu, gpu)
    
    Method generated by attrs for class Flavor.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    `name: beamlit.types.Unset | str`
    :

    `type_: beamlit.types.Unset | str`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`Form(config: beamlit.types.Unset | ForwardRef('FormConfig') = <beamlit.types.Unset object>, oauthomitempty: beamlit.types.Unset | ForwardRef('FormOauthomitempty') = <beamlit.types.Unset object>, secrets: beamlit.types.Unset | ForwardRef('FormSecrets') = <beamlit.types.Unset object>)`
:   Form of the artifact
    
    Attributes:
        config (Union[Unset, FormConfig]): Config of the artifact
        oauthomitempty (Union[Unset, FormOauthomitempty]): OAuth of the artifact
        secrets (Union[Unset, FormSecrets]): Secrets of the artifact
    
    Method generated by attrs for class Form.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties`
    :

    `config`
    :

    `oauthomitempty`
    :

    `secrets`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`FormConfig()`
:   Config of the artifact
    
    Method generated by attrs for class FormConfig.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`FormOauthomitempty()`
:   OAuth of the artifact
    
    Method generated by attrs for class FormOauthomitempty.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`FormSecrets()`
:   Secrets of the artifact
    
    Method generated by attrs for class FormSecrets.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`Function(events: beamlit.types.Unset | list['CoreEvent'] = <beamlit.types.Unset object>, metadata: beamlit.types.Unset | ForwardRef('Metadata') = <beamlit.types.Unset object>, spec: beamlit.types.Unset | ForwardRef('FunctionSpec') = <beamlit.types.Unset object>, status: beamlit.types.Unset | str = <beamlit.types.Unset object>)`
:   Function
    
    Attributes:
        events (Union[Unset, list['CoreEvent']]): Core events
        metadata (Union[Unset, Metadata]): Metadata
        spec (Union[Unset, FunctionSpec]): Function specification
        status (Union[Unset, str]): Function status
    
    Method generated by attrs for class Function.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties`
    :

    `events`
    :

    `metadata`
    :

    `spec`
    :

    `status`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`FunctionKit(description: beamlit.types.Unset | str = <beamlit.types.Unset object>, name: beamlit.types.Unset | str = <beamlit.types.Unset object>, parameters: beamlit.types.Unset | list['StoreFunctionParameter'] = <beamlit.types.Unset object>)`
:   Function kit
    
    Attributes:
        description (Union[Unset, str]): Description of the function kit, very important for the agent to work with your
            kit
        name (Union[Unset, str]): The kit name, very important for the agent to work with your kit
        parameters (Union[Unset, list['StoreFunctionParameter']]): Kit parameters, for your kit to be callable with an
            Agent
    
    Method generated by attrs for class FunctionKit.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties`
    :

    `description`
    :

    `name`
    :

    `parameters`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`FunctionSpec(configurations: beamlit.types.Unset | ForwardRef('CoreSpecConfigurations') = <beamlit.types.Unset object>, enabled: beamlit.types.Unset | bool = <beamlit.types.Unset object>, flavors: beamlit.types.Unset | list['Flavor'] = <beamlit.types.Unset object>, integration_connections: beamlit.types.Unset | list[str] = <beamlit.types.Unset object>, pod_template: beamlit.types.Unset | ForwardRef('PodTemplateSpec') = <beamlit.types.Unset object>, policies: beamlit.types.Unset | list[str] = <beamlit.types.Unset object>, private_clusters: beamlit.types.Unset | ForwardRef('ModelPrivateCluster') = <beamlit.types.Unset object>, runtime: beamlit.types.Unset | ForwardRef('Runtime') = <beamlit.types.Unset object>, sandbox: beamlit.types.Unset | bool = <beamlit.types.Unset object>, serverless_config: beamlit.types.Unset | ForwardRef('ServerlessConfig') = <beamlit.types.Unset object>, description: beamlit.types.Unset | str = <beamlit.types.Unset object>, kit: beamlit.types.Unset | list['FunctionKit'] = <beamlit.types.Unset object>, parameters: beamlit.types.Unset | list['StoreFunctionParameter'] = <beamlit.types.Unset object>, store_id: beamlit.types.Unset | str = <beamlit.types.Unset object>)`
:   Function specification
    
    Attributes:
        configurations (Union[Unset, CoreSpecConfigurations]): Optional configurations for the object
        enabled (Union[Unset, bool]): Enable or disable the agent
        flavors (Union[Unset, list['Flavor']]): Types of hardware available for deployments
        integration_connections (Union[Unset, list[str]]):
        pod_template (Union[Unset, PodTemplateSpec]): Pod template specification
        policies (Union[Unset, list[str]]):
        private_clusters (Union[Unset, ModelPrivateCluster]): Private cluster where the model deployment is deployed
        runtime (Union[Unset, Runtime]): Set of configurations for a deployment
        sandbox (Union[Unset, bool]): Sandbox mode
        serverless_config (Union[Unset, ServerlessConfig]): Configuration for a serverless deployment
        description (Union[Unset, str]): Function description, very important for the agent function to work with an LLM
        kit (Union[Unset, list['FunctionKit']]): The kit of the function deployment
        parameters (Union[Unset, list['StoreFunctionParameter']]): Function parameters, for your function to be callable
            with Agent
        store_id (Union[Unset, str]): Store id
    
    Method generated by attrs for class FunctionSpec.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties`
    :

    `configurations`
    :

    `description`
    :

    `enabled`
    :

    `flavors`
    :

    `integration_connections`
    :

    `kit`
    :

    `parameters`
    :

    `pod_template`
    :

    `policies`
    :

    `private_clusters`
    :

    `runtime`
    :

    `sandbox`
    :

    `serverless_config`
    :

    `store_id`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`GetTraceIdsResponse200()`
:   Method generated by attrs for class GetTraceIdsResponse200.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`GetTraceLogsResponse200()`
:   Method generated by attrs for class GetTraceLogsResponse200.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`GetTraceResponse200()`
:   Method generated by attrs for class GetTraceResponse200.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`GetWorkspaceServiceAccountsResponse200Item(client_id: beamlit.types.Unset | str = <beamlit.types.Unset object>, created_at: beamlit.types.Unset | str = <beamlit.types.Unset object>, description: beamlit.types.Unset | str = <beamlit.types.Unset object>, name: beamlit.types.Unset | str = <beamlit.types.Unset object>, updated_at: beamlit.types.Unset | str = <beamlit.types.Unset object>)`
:   Attributes:
        client_id (Union[Unset, str]): Service account client ID
        created_at (Union[Unset, str]): Creation timestamp
        description (Union[Unset, str]): Service account description
        name (Union[Unset, str]): Service account name
        updated_at (Union[Unset, str]): Last update timestamp
    
    Method generated by attrs for class GetWorkspaceServiceAccountsResponse200Item.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    `client_id: beamlit.types.Unset | str`
    :

    `created_at: beamlit.types.Unset | str`
    :

    `description: beamlit.types.Unset | str`
    :

    `name: beamlit.types.Unset | str`
    :

    `updated_at: beamlit.types.Unset | str`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`HistogramBucket(count: beamlit.types.Unset | int = <beamlit.types.Unset object>, end: beamlit.types.Unset | float = <beamlit.types.Unset object>, start: beamlit.types.Unset | float = <beamlit.types.Unset object>)`
:   Histogram bucket
    
    Attributes:
        count (Union[Unset, int]): Count
        end (Union[Unset, float]): End
        start (Union[Unset, float]): Start
    
    Method generated by attrs for class HistogramBucket.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    `count: beamlit.types.Unset | int`
    :

    `end: beamlit.types.Unset | float`
    :

    `start: beamlit.types.Unset | float`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`HistogramStats(average: beamlit.types.Unset | float = <beamlit.types.Unset object>, p50: beamlit.types.Unset | float = <beamlit.types.Unset object>, p90: beamlit.types.Unset | float = <beamlit.types.Unset object>, p99: beamlit.types.Unset | float = <beamlit.types.Unset object>)`
:   Histogram stats
    
    Attributes:
        average (Union[Unset, float]): Average request duration
        p50 (Union[Unset, float]): P50 request duration
        p90 (Union[Unset, float]): P90 request duration
        p99 (Union[Unset, float]): P99 request duration
    
    Method generated by attrs for class HistogramStats.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    `average: beamlit.types.Unset | float`
    :

    `p50: beamlit.types.Unset | float`
    :

    `p90: beamlit.types.Unset | float`
    :

    `p99: beamlit.types.Unset | float`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`IntegrationConnection(metadata: beamlit.types.Unset | ForwardRef('Metadata') = <beamlit.types.Unset object>, spec: beamlit.types.Unset | ForwardRef('IntegrationConnectionSpec') = <beamlit.types.Unset object>)`
:   Integration Connection
    
    Attributes:
        metadata (Union[Unset, Metadata]): Metadata
        spec (Union[Unset, IntegrationConnectionSpec]): Integration connection specification
    
    Method generated by attrs for class IntegrationConnection.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties`
    :

    `metadata`
    :

    `spec`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`IntegrationConnectionSpec(config: beamlit.types.Unset | ForwardRef('IntegrationConnectionSpecConfig') = <beamlit.types.Unset object>, integration: beamlit.types.Unset | str = <beamlit.types.Unset object>, sandbox: beamlit.types.Unset | bool = <beamlit.types.Unset object>, secret: beamlit.types.Unset | ForwardRef('IntegrationConnectionSpecSecret') = <beamlit.types.Unset object>)`
:   Integration connection specification
    
    Attributes:
        config (Union[Unset, IntegrationConnectionSpecConfig]): Additional configuration for the integration
        integration (Union[Unset, str]): Integration type
        sandbox (Union[Unset, bool]): Sandbox mode
        secret (Union[Unset, IntegrationConnectionSpecSecret]): Integration secret
    
    Method generated by attrs for class IntegrationConnectionSpec.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties`
    :

    `config`
    :

    `integration`
    :

    `sandbox`
    :

    `secret`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`IntegrationConnectionSpecConfig()`
:   Additional configuration for the integration
    
    Method generated by attrs for class IntegrationConnectionSpecConfig.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`IntegrationConnectionSpecSecret()`
:   Integration secret
    
    Method generated by attrs for class IntegrationConnectionSpecSecret.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`IntegrationModel(author: beamlit.types.Unset | str = <beamlit.types.Unset object>, created_at: beamlit.types.Unset | str = <beamlit.types.Unset object>, downloads: beamlit.types.Unset | int = <beamlit.types.Unset object>, endpoint: beamlit.types.Unset | str = <beamlit.types.Unset object>, id: beamlit.types.Unset | str = <beamlit.types.Unset object>, library_name: beamlit.types.Unset | str = <beamlit.types.Unset object>, likes: beamlit.types.Unset | int = <beamlit.types.Unset object>, model_private: beamlit.types.Unset | str = <beamlit.types.Unset object>, name: beamlit.types.Unset | str = <beamlit.types.Unset object>, pipeline_tag: beamlit.types.Unset | str = <beamlit.types.Unset object>, tags: beamlit.types.Unset | list[str] = <beamlit.types.Unset object>, trending_score: beamlit.types.Unset | int = <beamlit.types.Unset object>)`
:   Model obtained from an external authentication provider, such as HuggingFace, OpenAI, etc...
    
    Attributes:
        author (Union[Unset, str]): Provider model author
        created_at (Union[Unset, str]): Provider model created at
        downloads (Union[Unset, int]): Provider model downloads
        endpoint (Union[Unset, str]): Model endpoint URL
        id (Union[Unset, str]): Provider model ID
        library_name (Union[Unset, str]): Provider model library name
        likes (Union[Unset, int]): Provider model likes
        model_private (Union[Unset, str]): Is the model private
        name (Union[Unset, str]): Provider model name
        pipeline_tag (Union[Unset, str]): Provider model pipeline tag
        tags (Union[Unset, list[str]]): Provider model tags
        trending_score (Union[Unset, int]): Provider model trending score
    
    Method generated by attrs for class IntegrationModel.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    `author: beamlit.types.Unset | str`
    :

    `created_at: beamlit.types.Unset | str`
    :

    `downloads: beamlit.types.Unset | int`
    :

    `endpoint: beamlit.types.Unset | str`
    :

    `id: beamlit.types.Unset | str`
    :

    `library_name: beamlit.types.Unset | str`
    :

    `likes: beamlit.types.Unset | int`
    :

    `model_private: beamlit.types.Unset | str`
    :

    `name: beamlit.types.Unset | str`
    :

    `pipeline_tag: beamlit.types.Unset | str`
    :

    `tags: beamlit.types.Unset | list[str]`
    :

    `trending_score: beamlit.types.Unset | int`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`IntegrationRepository(id: beamlit.types.Unset | str = <beamlit.types.Unset object>, name: beamlit.types.Unset | str = <beamlit.types.Unset object>, organization: beamlit.types.Unset | str = <beamlit.types.Unset object>, url: beamlit.types.Unset | str = <beamlit.types.Unset object>)`
:   Integration repository
    
    Attributes:
        id (Union[Unset, str]): Repository ID
        name (Union[Unset, str]): Repository name
        organization (Union[Unset, str]): Repository owner
        url (Union[Unset, str]): Repository URL
    
    Method generated by attrs for class IntegrationRepository.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    `id: beamlit.types.Unset | str`
    :

    `name: beamlit.types.Unset | str`
    :

    `organization: beamlit.types.Unset | str`
    :

    `url: beamlit.types.Unset | str`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`InviteWorkspaceUserBody(email: beamlit.types.Unset | str = <beamlit.types.Unset object>)`
:   Attributes:
        email (Union[Unset, str]):
    
    Method generated by attrs for class InviteWorkspaceUserBody.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    `email: beamlit.types.Unset | str`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`Knowledgebase(events: beamlit.types.Unset | list['CoreEvent'] = <beamlit.types.Unset object>, metadata: beamlit.types.Unset | ForwardRef('Metadata') = <beamlit.types.Unset object>, spec: beamlit.types.Unset | ForwardRef('KnowledgebaseSpec') = <beamlit.types.Unset object>, status: beamlit.types.Unset | str = <beamlit.types.Unset object>)`
:   Knowledgebase
    
    Attributes:
        events (Union[Unset, list['CoreEvent']]): Core events
        metadata (Union[Unset, Metadata]): Metadata
        spec (Union[Unset, KnowledgebaseSpec]): Knowledgebase specification
        status (Union[Unset, str]): Knowledgebase status
    
    Method generated by attrs for class Knowledgebase.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties`
    :

    `events`
    :

    `metadata`
    :

    `spec`
    :

    `status`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`KnowledgebaseSpec(collection_name: beamlit.types.Unset | str = <beamlit.types.Unset object>, embedding_model: beamlit.types.Unset | str = <beamlit.types.Unset object>, embedding_model_type: beamlit.types.Unset | str = <beamlit.types.Unset object>, enabled: beamlit.types.Unset | bool = <beamlit.types.Unset object>, integration_connections: beamlit.types.Unset | list[str] = <beamlit.types.Unset object>, options: beamlit.types.Unset | ForwardRef('KnowledgebaseSpecOptions') = <beamlit.types.Unset object>, policies: beamlit.types.Unset | list[str] = <beamlit.types.Unset object>, sandbox: beamlit.types.Unset | bool = <beamlit.types.Unset object>)`
:   Knowledgebase specification
    
    Attributes:
        collection_name (Union[Unset, str]): Collection name
        embedding_model (Union[Unset, str]): Embedding model
        embedding_model_type (Union[Unset, str]): Embedding model type
        enabled (Union[Unset, bool]): Enable or disable the agent
        integration_connections (Union[Unset, list[str]]):
        options (Union[Unset, KnowledgebaseSpecOptions]): Options specific to the knowledge base
        policies (Union[Unset, list[str]]):
        sandbox (Union[Unset, bool]): Sandbox mode
    
    Method generated by attrs for class KnowledgebaseSpec.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties`
    :

    `collection_name`
    :

    `embedding_model`
    :

    `embedding_model_type`
    :

    `enabled`
    :

    `integration_connections`
    :

    `options`
    :

    `policies`
    :

    `sandbox`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`KnowledgebaseSpecOptions()`
:   Options specific to the knowledge base
    
    Method generated by attrs for class KnowledgebaseSpecOptions.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`LastNRequestsMetric(date: beamlit.types.Unset | str = <beamlit.types.Unset object>, workload_type: beamlit.types.Unset | str = <beamlit.types.Unset object>, workspace: beamlit.types.Unset | str = <beamlit.types.Unset object>)`
:   Last N requests
    
    Attributes:
        date (Union[Unset, str]): Timestamp
        workload_type (Union[Unset, str]): Workload type
        workspace (Union[Unset, str]): Workspace
    
    Method generated by attrs for class LastNRequestsMetric.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    `date: beamlit.types.Unset | str`
    :

    `workload_type: beamlit.types.Unset | str`
    :

    `workspace: beamlit.types.Unset | str`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`LatencyMetric(global_histogram: beamlit.types.Unset | ForwardRef('HistogramBucket') = <beamlit.types.Unset object>, global_stats: beamlit.types.Unset | ForwardRef('HistogramStats') = <beamlit.types.Unset object>, histogram_per_code: beamlit.types.Unset | ForwardRef('HistogramBucket') = <beamlit.types.Unset object>, stats_per_code: beamlit.types.Unset | ForwardRef('HistogramStats') = <beamlit.types.Unset object>)`
:   Latency metrics
    
    Attributes:
        global_histogram (Union[Unset, HistogramBucket]): Histogram bucket
        global_stats (Union[Unset, HistogramStats]): Histogram stats
        histogram_per_code (Union[Unset, HistogramBucket]): Histogram bucket
        stats_per_code (Union[Unset, HistogramStats]): Histogram stats
    
    Method generated by attrs for class LatencyMetric.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties`
    :

    `global_histogram`
    :

    `global_stats`
    :

    `histogram_per_code`
    :

    `stats_per_code`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`LocationResponse(continent: beamlit.types.Unset | str = <beamlit.types.Unset object>, country: beamlit.types.Unset | str = <beamlit.types.Unset object>, flavors: beamlit.types.Unset | list['Flavor'] = <beamlit.types.Unset object>, location: beamlit.types.Unset | str = <beamlit.types.Unset object>, status: beamlit.types.Unset | str = <beamlit.types.Unset object>)`
:   Location availability for policies
    
    Attributes:
        continent (Union[Unset, str]): Continent of the location
        country (Union[Unset, str]): Country of the location
        flavors (Union[Unset, list['Flavor']]): Hardware flavors available in the location
        location (Union[Unset, str]): Name of the location
        status (Union[Unset, str]): Status of the location
    
    Method generated by attrs for class LocationResponse.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties`
    :

    `continent`
    :

    `country`
    :

    `flavors`
    :

    `location`
    :

    `status`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`MCPDefinition(categories: beamlit.types.Unset | list[typing.Any] = <beamlit.types.Unset object>, coming_soon: beamlit.types.Unset | bool = <beamlit.types.Unset object>, description: beamlit.types.Unset | str = <beamlit.types.Unset object>, display_name: beamlit.types.Unset | str = <beamlit.types.Unset object>, enterprise: beamlit.types.Unset | bool = <beamlit.types.Unset object>, entrypoint: beamlit.types.Unset | ForwardRef('MCPDefinitionEntrypoint') = <beamlit.types.Unset object>, form: beamlit.types.Unset | ForwardRef('MCPDefinitionForm') = <beamlit.types.Unset object>, icon: beamlit.types.Unset | str = <beamlit.types.Unset object>, integration: beamlit.types.Unset | str = <beamlit.types.Unset object>, long_description: beamlit.types.Unset | str = <beamlit.types.Unset object>, name: beamlit.types.Unset | str = <beamlit.types.Unset object>, url: beamlit.types.Unset | str = <beamlit.types.Unset object>)`
:   Definition of an MCP from the MCP Hub
    
    Attributes:
        categories (Union[Unset, list[Any]]): Categories of the artifact
        coming_soon (Union[Unset, bool]): If the artifact is coming soon
        description (Union[Unset, str]): Description of the artifact
        display_name (Union[Unset, str]): Display name of the artifact
        enterprise (Union[Unset, bool]): If the artifact is enterprise
        entrypoint (Union[Unset, MCPDefinitionEntrypoint]): Entrypoint of the artifact
        form (Union[Unset, MCPDefinitionForm]): Form of the artifact
        icon (Union[Unset, str]): Icon of the artifact
        integration (Union[Unset, str]): Integration of the artifact
        long_description (Union[Unset, str]): Long description of the artifact
        name (Union[Unset, str]): Name of the artifact
        url (Union[Unset, str]): URL of the artifact
    
    Method generated by attrs for class MCPDefinition.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties`
    :

    `categories`
    :

    `coming_soon`
    :

    `description`
    :

    `display_name`
    :

    `enterprise`
    :

    `entrypoint`
    :

    `form`
    :

    `icon`
    :

    `integration`
    :

    `long_description`
    :

    `name`
    :

    `url`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`MCPDefinitionEntrypoint()`
:   Entrypoint of the artifact
    
    Method generated by attrs for class MCPDefinitionEntrypoint.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`MCPDefinitionForm()`
:   Form of the artifact
    
    Method generated by attrs for class MCPDefinitionForm.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`Metadata(created_at: beamlit.types.Unset | str = <beamlit.types.Unset object>, updated_at: beamlit.types.Unset | str = <beamlit.types.Unset object>, created_by: beamlit.types.Unset | str = <beamlit.types.Unset object>, updated_by: beamlit.types.Unset | str = <beamlit.types.Unset object>, display_name: beamlit.types.Unset | str = <beamlit.types.Unset object>, labels: beamlit.types.Unset | ForwardRef('MetadataLabels') = <beamlit.types.Unset object>, name: beamlit.types.Unset | str = <beamlit.types.Unset object>, workspace: beamlit.types.Unset | str = <beamlit.types.Unset object>)`
:   Metadata
    
    Attributes:
        created_at (Union[Unset, str]): The date and time when the resource was created
        updated_at (Union[Unset, str]): The date and time when the resource was updated
        created_by (Union[Unset, str]): The user or service account who created the resource
        updated_by (Union[Unset, str]): The user or service account who updated the resource
        display_name (Union[Unset, str]): Model display name
        labels (Union[Unset, MetadataLabels]): Labels
        name (Union[Unset, str]): Model name
        workspace (Union[Unset, str]): Workspace name
    
    Method generated by attrs for class Metadata.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties`
    :

    `created_at`
    :

    `created_by`
    :

    `display_name`
    :

    `labels`
    :

    `name`
    :

    `updated_at`
    :

    `updated_by`
    :

    `workspace`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`MetadataLabels()`
:   Labels
    
    Method generated by attrs for class MetadataLabels.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, str]`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`Metric(rate: beamlit.types.Unset | int = <beamlit.types.Unset object>, request_total: beamlit.types.Unset | int = <beamlit.types.Unset object>, timestamp: beamlit.types.Unset | str = <beamlit.types.Unset object>)`
:   Metric
    
    Attributes:
        rate (Union[Unset, int]): Metric value
        request_total (Union[Unset, int]): Metric value
        timestamp (Union[Unset, str]): Metric timestamp
    
    Method generated by attrs for class Metric.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    `rate: beamlit.types.Unset | int`
    :

    `request_total: beamlit.types.Unset | int`
    :

    `timestamp: beamlit.types.Unset | str`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`Metrics(agents: beamlit.types.Unset | Any = <beamlit.types.Unset object>, functions: beamlit.types.Unset | Any = <beamlit.types.Unset object>, inference_global: beamlit.types.Unset | list[typing.Any] = <beamlit.types.Unset object>, models: beamlit.types.Unset | ForwardRef('MetricsModels') = <beamlit.types.Unset object>, request_total: beamlit.types.Unset | float = <beamlit.types.Unset object>, request_total_per_code: beamlit.types.Unset | ForwardRef('MetricsRequestTotalPerCode') = <beamlit.types.Unset object>, rps: beamlit.types.Unset | float = <beamlit.types.Unset object>, rps_per_code: beamlit.types.Unset | ForwardRef('MetricsRpsPerCode') = <beamlit.types.Unset object>)`
:   Metrics for resources
    
    Attributes:
        agents (Union[Unset, Any]): Metrics for agents
        functions (Union[Unset, Any]): Metrics for functions
        inference_global (Union[Unset, list[Any]]): Historical requests for all resources globally
        models (Union[Unset, MetricsModels]): Metrics for models
        request_total (Union[Unset, float]): Number of requests for all resources globally
        request_total_per_code (Union[Unset, MetricsRequestTotalPerCode]): Number of requests for all resources globally
            per code
        rps (Union[Unset, float]): Number of requests per second for all resources globally
        rps_per_code (Union[Unset, MetricsRpsPerCode]): Number of requests per second for all resources globally per
            code
    
    Method generated by attrs for class Metrics.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties`
    :

    `agents`
    :

    `functions`
    :

    `inference_global`
    :

    `models`
    :

    `request_total`
    :

    `request_total_per_code`
    :

    `rps`
    :

    `rps_per_code`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`MetricsModels()`
:   Metrics for models
    
    Method generated by attrs for class MetricsModels.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`MetricsRequestTotalPerCode()`
:   Number of requests for all resources globally per code
    
    Method generated by attrs for class MetricsRequestTotalPerCode.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`MetricsRpsPerCode()`
:   Number of requests per second for all resources globally per code
    
    Method generated by attrs for class MetricsRpsPerCode.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`Model(events: beamlit.types.Unset | list['CoreEvent'] = <beamlit.types.Unset object>, metadata: beamlit.types.Unset | ForwardRef('Metadata') = <beamlit.types.Unset object>, spec: beamlit.types.Unset | ForwardRef('ModelSpec') = <beamlit.types.Unset object>, status: beamlit.types.Unset | str = <beamlit.types.Unset object>)`
:   Logical object representing a model
    
    Attributes:
        events (Union[Unset, list['CoreEvent']]): Core events
        metadata (Union[Unset, Metadata]): Metadata
        spec (Union[Unset, ModelSpec]): Model specification
        status (Union[Unset, str]): Model status
    
    Method generated by attrs for class Model.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties`
    :

    `events`
    :

    `metadata`
    :

    `spec`
    :

    `status`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`ModelPrivateCluster(base_url: beamlit.types.Unset | str = <beamlit.types.Unset object>, enabled: beamlit.types.Unset | bool = <beamlit.types.Unset object>, name: beamlit.types.Unset | str = <beamlit.types.Unset object>)`
:   Private cluster where the model deployment is deployed
    
    Attributes:
        base_url (Union[Unset, str]): The base url of the model in the private cluster
        enabled (Union[Unset, bool]): If true, the private cluster is available
        name (Union[Unset, str]): The name of the private cluster
    
    Method generated by attrs for class ModelPrivateCluster.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    `base_url: beamlit.types.Unset | str`
    :

    `enabled: beamlit.types.Unset | bool`
    :

    `name: beamlit.types.Unset | str`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`ModelSpec(configurations: beamlit.types.Unset | ForwardRef('CoreSpecConfigurations') = <beamlit.types.Unset object>, enabled: beamlit.types.Unset | bool = <beamlit.types.Unset object>, flavors: beamlit.types.Unset | list['Flavor'] = <beamlit.types.Unset object>, integration_connections: beamlit.types.Unset | list[str] = <beamlit.types.Unset object>, pod_template: beamlit.types.Unset | ForwardRef('PodTemplateSpec') = <beamlit.types.Unset object>, policies: beamlit.types.Unset | list[str] = <beamlit.types.Unset object>, private_clusters: beamlit.types.Unset | ForwardRef('ModelPrivateCluster') = <beamlit.types.Unset object>, runtime: beamlit.types.Unset | ForwardRef('Runtime') = <beamlit.types.Unset object>, sandbox: beamlit.types.Unset | bool = <beamlit.types.Unset object>, serverless_config: beamlit.types.Unset | ForwardRef('ServerlessConfig') = <beamlit.types.Unset object>)`
:   Model specification
    
    Attributes:
        configurations (Union[Unset, CoreSpecConfigurations]): Optional configurations for the object
        enabled (Union[Unset, bool]): Enable or disable the agent
        flavors (Union[Unset, list['Flavor']]): Types of hardware available for deployments
        integration_connections (Union[Unset, list[str]]):
        pod_template (Union[Unset, PodTemplateSpec]): Pod template specification
        policies (Union[Unset, list[str]]):
        private_clusters (Union[Unset, ModelPrivateCluster]): Private cluster where the model deployment is deployed
        runtime (Union[Unset, Runtime]): Set of configurations for a deployment
        sandbox (Union[Unset, bool]): Sandbox mode
        serverless_config (Union[Unset, ServerlessConfig]): Configuration for a serverless deployment
    
    Method generated by attrs for class ModelSpec.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties`
    :

    `configurations`
    :

    `enabled`
    :

    `flavors`
    :

    `integration_connections`
    :

    `pod_template`
    :

    `policies`
    :

    `private_clusters`
    :

    `runtime`
    :

    `sandbox`
    :

    `serverless_config`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`OAuth(scope: beamlit.types.Unset | list[typing.Any] = <beamlit.types.Unset object>, type_: beamlit.types.Unset | str = <beamlit.types.Unset object>)`
:   OAuth of the artifact
    
    Attributes:
        scope (Union[Unset, list[Any]]): Scope of the OAuth
        type_ (Union[Unset, str]): Type of the OAuth
    
    Method generated by attrs for class OAuth.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    `scope: beamlit.types.Unset | list[typing.Any]`
    :

    `type_: beamlit.types.Unset | str`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`OwnerFields(created_by: beamlit.types.Unset | str = <beamlit.types.Unset object>, updated_by: beamlit.types.Unset | str = <beamlit.types.Unset object>)`
:   Owner fields for Persistance
    
    Attributes:
        created_by (Union[Unset, str]): The user or service account who created the resource
        updated_by (Union[Unset, str]): The user or service account who updated the resource
    
    Method generated by attrs for class OwnerFields.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    `created_by: beamlit.types.Unset | str`
    :

    `updated_by: beamlit.types.Unset | str`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`PendingInvitation(created_at: beamlit.types.Unset | str = <beamlit.types.Unset object>, updated_at: beamlit.types.Unset | str = <beamlit.types.Unset object>, created_by: beamlit.types.Unset | str = <beamlit.types.Unset object>, updated_by: beamlit.types.Unset | str = <beamlit.types.Unset object>, email: beamlit.types.Unset | str = <beamlit.types.Unset object>, invited_by: beamlit.types.Unset | str = <beamlit.types.Unset object>, role: beamlit.types.Unset | str = <beamlit.types.Unset object>, workspace: beamlit.types.Unset | str = <beamlit.types.Unset object>)`
:   Pending invitation in workspace
    
    Attributes:
        created_at (Union[Unset, str]): The date and time when the resource was created
        updated_at (Union[Unset, str]): The date and time when the resource was updated
        created_by (Union[Unset, str]): The user or service account who created the resource
        updated_by (Union[Unset, str]): The user or service account who updated the resource
        email (Union[Unset, str]): User email
        invited_by (Union[Unset, str]): User sub
        role (Union[Unset, str]): ACL role
        workspace (Union[Unset, str]): Workspace name
    
    Method generated by attrs for class PendingInvitation.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    `created_at: beamlit.types.Unset | str`
    :

    `created_by: beamlit.types.Unset | str`
    :

    `email: beamlit.types.Unset | str`
    :

    `invited_by: beamlit.types.Unset | str`
    :

    `role: beamlit.types.Unset | str`
    :

    `updated_at: beamlit.types.Unset | str`
    :

    `updated_by: beamlit.types.Unset | str`
    :

    `workspace: beamlit.types.Unset | str`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`PendingInvitationAccept(email: beamlit.types.Unset | str = <beamlit.types.Unset object>, workspace: beamlit.types.Unset | ForwardRef('Workspace') = <beamlit.types.Unset object>)`
:   Pending invitation accept
    
    Attributes:
        email (Union[Unset, str]): User email
        workspace (Union[Unset, Workspace]): Workspace
    
    Method generated by attrs for class PendingInvitationAccept.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties`
    :

    `email`
    :

    `workspace`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`PendingInvitationRender(email: beamlit.types.Unset | str = <beamlit.types.Unset object>, invited_at: beamlit.types.Unset | str = <beamlit.types.Unset object>, invited_by: beamlit.types.Unset | ForwardRef('PendingInvitationRenderInvitedBy') = <beamlit.types.Unset object>, role: beamlit.types.Unset | str = <beamlit.types.Unset object>, workspace: beamlit.types.Unset | ForwardRef('PendingInvitationRenderWorkspace') = <beamlit.types.Unset object>, workspace_details: beamlit.types.Unset | ForwardRef('PendingInvitationWorkspaceDetails') = <beamlit.types.Unset object>)`
:   Pending invitation in workspace
    
    Attributes:
        email (Union[Unset, str]): User email
        invited_at (Union[Unset, str]): Invitation date
        invited_by (Union[Unset, PendingInvitationRenderInvitedBy]): Invited by
        role (Union[Unset, str]): ACL role
        workspace (Union[Unset, PendingInvitationRenderWorkspace]): Workspace
        workspace_details (Union[Unset, PendingInvitationWorkspaceDetails]): Workspace details
    
    Method generated by attrs for class PendingInvitationRender.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties`
    :

    `email`
    :

    `invited_at`
    :

    `invited_by`
    :

    `role`
    :

    `workspace`
    :

    `workspace_details`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`PendingInvitationRenderInvitedBy(email: beamlit.types.Unset | str = <beamlit.types.Unset object>, family_name: beamlit.types.Unset | str = <beamlit.types.Unset object>, given_name: beamlit.types.Unset | str = <beamlit.types.Unset object>, sub: beamlit.types.Unset | str = <beamlit.types.Unset object>)`
:   Invited by
    
    Attributes:
        email (Union[Unset, str]): User email
        family_name (Union[Unset, str]): User family name
        given_name (Union[Unset, str]): User given name
        sub (Union[Unset, str]): User sub
    
    Method generated by attrs for class PendingInvitationRenderInvitedBy.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    `email: beamlit.types.Unset | str`
    :

    `family_name: beamlit.types.Unset | str`
    :

    `given_name: beamlit.types.Unset | str`
    :

    `sub: beamlit.types.Unset | str`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`PendingInvitationRenderWorkspace(display_name: beamlit.types.Unset | str = <beamlit.types.Unset object>, name: beamlit.types.Unset | str = <beamlit.types.Unset object>)`
:   Workspace
    
    Attributes:
        display_name (Union[Unset, str]): Workspace display name
        name (Union[Unset, str]): Workspace name
    
    Method generated by attrs for class PendingInvitationRenderWorkspace.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    `display_name: beamlit.types.Unset | str`
    :

    `name: beamlit.types.Unset | str`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`PendingInvitationWorkspaceDetails(emails: beamlit.types.Unset | list[typing.Any] = <beamlit.types.Unset object>, user_number: beamlit.types.Unset | float = <beamlit.types.Unset object>)`
:   Workspace details
    
    Attributes:
        emails (Union[Unset, list[Any]]): List of user emails in the workspace
        user_number (Union[Unset, float]): Number of users in the workspace
    
    Method generated by attrs for class PendingInvitationWorkspaceDetails.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    `emails: beamlit.types.Unset | list[typing.Any]`
    :

    `user_number: beamlit.types.Unset | float`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`PodTemplateSpec()`
:   Pod template specification
    
    Method generated by attrs for class PodTemplateSpec.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`Policy(metadata: beamlit.types.Unset | ForwardRef('Metadata') = <beamlit.types.Unset object>, spec: beamlit.types.Unset | ForwardRef('PolicySpec') = <beamlit.types.Unset object>)`
:   Rule that controls how a deployment is made and served (e.g. location restrictions)
    
    Attributes:
        metadata (Union[Unset, Metadata]): Metadata
        spec (Union[Unset, PolicySpec]): Policy specification
    
    Method generated by attrs for class Policy.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties`
    :

    `metadata`
    :

    `spec`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`PolicyLocation(name: beamlit.types.Unset | str = <beamlit.types.Unset object>, type_: beamlit.types.Unset | str = <beamlit.types.Unset object>)`
:   Policy location
    
    Attributes:
        name (Union[Unset, str]): Policy location name
        type_ (Union[Unset, str]): Policy location type
    
    Method generated by attrs for class PolicyLocation.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    `name: beamlit.types.Unset | str`
    :

    `type_: beamlit.types.Unset | str`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`PolicyMaxTokens(granularity: beamlit.types.Unset | str = <beamlit.types.Unset object>, input_: beamlit.types.Unset | int = <beamlit.types.Unset object>, output: beamlit.types.Unset | int = <beamlit.types.Unset object>, ratio_input_over_output: beamlit.types.Unset | int = <beamlit.types.Unset object>, step: beamlit.types.Unset | int = <beamlit.types.Unset object>, total: beamlit.types.Unset | int = <beamlit.types.Unset object>)`
:   PolicyMaxTokens is a local type that wraps a slice of PolicyMaxTokens
    
    Attributes:
        granularity (Union[Unset, str]): Granularity
        input_ (Union[Unset, int]): Input
        output (Union[Unset, int]): Output
        ratio_input_over_output (Union[Unset, int]): RatioInputOverOutput
        step (Union[Unset, int]): Step
        total (Union[Unset, int]): Total
    
    Method generated by attrs for class PolicyMaxTokens.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    `granularity: beamlit.types.Unset | str`
    :

    `input_: beamlit.types.Unset | int`
    :

    `output: beamlit.types.Unset | int`
    :

    `ratio_input_over_output: beamlit.types.Unset | int`
    :

    `step: beamlit.types.Unset | int`
    :

    `total: beamlit.types.Unset | int`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`PolicySpec(flavors: beamlit.types.Unset | list['Flavor'] = <beamlit.types.Unset object>, locations: beamlit.types.Unset | list['PolicyLocation'] = <beamlit.types.Unset object>, max_tokens: beamlit.types.Unset | ForwardRef('PolicyMaxTokens') = <beamlit.types.Unset object>, resource_types: beamlit.types.Unset | list[str] = <beamlit.types.Unset object>, sandbox: beamlit.types.Unset | bool = <beamlit.types.Unset object>, type_: beamlit.types.Unset | str = <beamlit.types.Unset object>)`
:   Policy specification
    
    Attributes:
        flavors (Union[Unset, list['Flavor']]): Types of hardware available for deployments
        locations (Union[Unset, list['PolicyLocation']]): PolicyLocations is a local type that wraps a slice of Location
        max_tokens (Union[Unset, PolicyMaxTokens]): PolicyMaxTokens is a local type that wraps a slice of
            PolicyMaxTokens
        resource_types (Union[Unset, list[str]]): PolicyResourceTypes is a local type that wraps a slice of
            PolicyResourceType
        sandbox (Union[Unset, bool]): Sandbox mode
        type_ (Union[Unset, str]): Policy type, can be location or flavor
    
    Method generated by attrs for class PolicySpec.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties`
    :

    `flavors`
    :

    `locations`
    :

    `max_tokens`
    :

    `resource_types`
    :

    `sandbox`
    :

    `type_`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`PrivateCluster(created_at: beamlit.types.Unset | str = <beamlit.types.Unset object>, updated_at: beamlit.types.Unset | str = <beamlit.types.Unset object>, created_by: beamlit.types.Unset | str = <beamlit.types.Unset object>, updated_by: beamlit.types.Unset | str = <beamlit.types.Unset object>, continent: beamlit.types.Unset | str = <beamlit.types.Unset object>, country: beamlit.types.Unset | str = <beamlit.types.Unset object>, display_name: beamlit.types.Unset | str = <beamlit.types.Unset object>, healthy: beamlit.types.Unset | bool = <beamlit.types.Unset object>, last_health_check_time: beamlit.types.Unset | str = <beamlit.types.Unset object>, latitude: beamlit.types.Unset | str = <beamlit.types.Unset object>, longitude: beamlit.types.Unset | str = <beamlit.types.Unset object>, name: beamlit.types.Unset | str = <beamlit.types.Unset object>, owned_by: beamlit.types.Unset | str = <beamlit.types.Unset object>, workspace: beamlit.types.Unset | str = <beamlit.types.Unset object>)`
:   A private cluster where models can be located on.
    
    Attributes:
        created_at (Union[Unset, str]): The date and time when the resource was created
        updated_at (Union[Unset, str]): The date and time when the resource was updated
        created_by (Union[Unset, str]): The user or service account who created the resource
        updated_by (Union[Unset, str]): The user or service account who updated the resource
        continent (Union[Unset, str]): The private cluster's continent, used to determine the closest private cluster to
            serve inference requests based on the user's location
        country (Union[Unset, str]): The country where the private cluster is located, used to determine the closest
            private cluster to serve inference requests based on the user's location
        display_name (Union[Unset, str]): The private cluster's display Name
        healthy (Union[Unset, bool]): Whether the private cluster is healthy or not, used to determine if the private
            cluster is ready to run inference
        last_health_check_time (Union[Unset, str]): The private cluster's unique name
        latitude (Union[Unset, str]): The private cluster's latitude, used to determine the closest private cluster to
            serve inference requests based on the user's location
        longitude (Union[Unset, str]): The private cluster's longitude, used to determine the closest private cluster to
            serve inference requests based on the user's location
        name (Union[Unset, str]): The name of the private cluster, it must be unique
        owned_by (Union[Unset, str]): The service account (operator) that owns the cluster
        workspace (Union[Unset, str]): The workspace the private cluster belongs to
    
    Method generated by attrs for class PrivateCluster.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    `continent: beamlit.types.Unset | str`
    :

    `country: beamlit.types.Unset | str`
    :

    `created_at: beamlit.types.Unset | str`
    :

    `created_by: beamlit.types.Unset | str`
    :

    `display_name: beamlit.types.Unset | str`
    :

    `healthy: beamlit.types.Unset | bool`
    :

    `last_health_check_time: beamlit.types.Unset | str`
    :

    `latitude: beamlit.types.Unset | str`
    :

    `longitude: beamlit.types.Unset | str`
    :

    `name: beamlit.types.Unset | str`
    :

    `owned_by: beamlit.types.Unset | str`
    :

    `updated_at: beamlit.types.Unset | str`
    :

    `updated_by: beamlit.types.Unset | str`
    :

    `workspace: beamlit.types.Unset | str`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`PrivateLocation(name: beamlit.types.Unset | str = <beamlit.types.Unset object>)`
:   Private location available for policies
    
    Attributes:
        name (Union[Unset, str]): Location name
    
    Method generated by attrs for class PrivateLocation.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    `name: beamlit.types.Unset | str`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`Repository(type_: beamlit.types.Unset | str = <beamlit.types.Unset object>, url: beamlit.types.Unset | str = <beamlit.types.Unset object>)`
:   Repository
    
    Attributes:
        type_ (Union[Unset, str]): Repository type
        url (Union[Unset, str]): Repository URL
    
    Method generated by attrs for class Repository.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    `type_: beamlit.types.Unset | str`
    :

    `url: beamlit.types.Unset | str`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`RequestDurationOverTimeMetric(average: beamlit.types.Unset | float = <beamlit.types.Unset object>, p50: beamlit.types.Unset | float = <beamlit.types.Unset object>, p90: beamlit.types.Unset | float = <beamlit.types.Unset object>, p99: beamlit.types.Unset | float = <beamlit.types.Unset object>, timestamp: beamlit.types.Unset | str = <beamlit.types.Unset object>)`
:   Request duration over time metric
    
    Attributes:
        average (Union[Unset, float]): Average request duration
        p50 (Union[Unset, float]): P50 request duration
        p90 (Union[Unset, float]): P90 request duration
        p99 (Union[Unset, float]): P99 request duration
        timestamp (Union[Unset, str]): Timestamp
    
    Method generated by attrs for class RequestDurationOverTimeMetric.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    `average: beamlit.types.Unset | float`
    :

    `p50: beamlit.types.Unset | float`
    :

    `p90: beamlit.types.Unset | float`
    :

    `p99: beamlit.types.Unset | float`
    :

    `timestamp: beamlit.types.Unset | str`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`RequestDurationOverTimeMetrics(request_duration_over_time: beamlit.types.Unset | ForwardRef('RequestDurationOverTimeMetric') = <beamlit.types.Unset object>)`
:   Request duration over time metrics
    
    Attributes:
        request_duration_over_time (Union[Unset, RequestDurationOverTimeMetric]): Request duration over time metric
    
    Method generated by attrs for class RequestDurationOverTimeMetrics.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties`
    :

    `request_duration_over_time`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`RequestTotalByOriginMetric(request_total_by_origin: beamlit.types.Unset | ForwardRef('RequestTotalByOriginMetricRequestTotalByOrigin') = <beamlit.types.Unset object>, request_total_by_origin_and_code: beamlit.types.Unset | ForwardRef('RequestTotalByOriginMetricRequestTotalByOriginAndCode') = <beamlit.types.Unset object>)`
:   Request total by origin metric
    
    Attributes:
        request_total_by_origin (Union[Unset, RequestTotalByOriginMetricRequestTotalByOrigin]): Request total by origin
        request_total_by_origin_and_code (Union[Unset, RequestTotalByOriginMetricRequestTotalByOriginAndCode]): Request
            total by origin and code
    
    Method generated by attrs for class RequestTotalByOriginMetric.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties`
    :

    `request_total_by_origin`
    :

    `request_total_by_origin_and_code`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`RequestTotalByOriginMetricRequestTotalByOrigin()`
:   Request total by origin
    
    Method generated by attrs for class RequestTotalByOriginMetricRequestTotalByOrigin.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`RequestTotalByOriginMetricRequestTotalByOriginAndCode()`
:   Request total by origin and code
    
    Method generated by attrs for class RequestTotalByOriginMetricRequestTotalByOriginAndCode.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`RequestTotalMetric(request_total: beamlit.types.Unset | float = <beamlit.types.Unset object>, request_total_per_code: beamlit.types.Unset | ForwardRef('RequestTotalMetricRequestTotalPerCode') = <beamlit.types.Unset object>, rps: beamlit.types.Unset | float = <beamlit.types.Unset object>, rps_per_code: beamlit.types.Unset | ForwardRef('RequestTotalMetricRpsPerCode') = <beamlit.types.Unset object>)`
:   Metrics for request total
    
    Attributes:
        request_total (Union[Unset, float]): Number of requests for all resources globally
        request_total_per_code (Union[Unset, RequestTotalMetricRequestTotalPerCode]): Number of requests for all
            resources globally per code
        rps (Union[Unset, float]): Number of requests per second for all resources globally
        rps_per_code (Union[Unset, RequestTotalMetricRpsPerCode]): Number of requests for all resources globally
    
    Method generated by attrs for class RequestTotalMetric.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties`
    :

    `request_total`
    :

    `request_total_per_code`
    :

    `rps`
    :

    `rps_per_code`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`RequestTotalMetricRequestTotalPerCode()`
:   Number of requests for all resources globally per code
    
    Method generated by attrs for class RequestTotalMetricRequestTotalPerCode.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`RequestTotalMetricRpsPerCode()`
:   Number of requests for all resources globally
    
    Method generated by attrs for class RequestTotalMetricRpsPerCode.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`ResourceLog(message: beamlit.types.Unset | str = <beamlit.types.Unset object>, severity: beamlit.types.Unset | int = <beamlit.types.Unset object>, timestamp: beamlit.types.Unset | str = <beamlit.types.Unset object>)`
:   Log for a resource deployment (eg. model deployment, function deployment)
    
    Attributes:
        message (Union[Unset, str]): Content of the log
        severity (Union[Unset, int]): Severity of the log
        timestamp (Union[Unset, str]): The timestamp of the log
    
    Method generated by attrs for class ResourceLog.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    `message: beamlit.types.Unset | str`
    :

    `severity: beamlit.types.Unset | int`
    :

    `timestamp: beamlit.types.Unset | str`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`ResourceMetrics(inference_global: beamlit.types.Unset | list['Metric'] = <beamlit.types.Unset object>, last_n_requests: beamlit.types.Unset | list['Metric'] = <beamlit.types.Unset object>, latency: beamlit.types.Unset | ForwardRef('LatencyMetric') = <beamlit.types.Unset object>, request_duration_over_time: beamlit.types.Unset | ForwardRef('RequestDurationOverTimeMetrics') = <beamlit.types.Unset object>, request_total: beamlit.types.Unset | float = <beamlit.types.Unset object>, request_total_by_origin: beamlit.types.Unset | ForwardRef('RequestTotalByOriginMetric') = <beamlit.types.Unset object>, request_total_per_code: beamlit.types.Unset | ForwardRef('ResourceMetricsRequestTotalPerCode') = <beamlit.types.Unset object>, rps: beamlit.types.Unset | float = <beamlit.types.Unset object>, rps_per_code: beamlit.types.Unset | ForwardRef('ResourceMetricsRpsPerCode') = <beamlit.types.Unset object>, token_rate: beamlit.types.Unset | ForwardRef('TokenRateMetrics') = <beamlit.types.Unset object>, token_total: beamlit.types.Unset | ForwardRef('TokenTotalMetric') = <beamlit.types.Unset object>)`
:   Metrics for a single resource deployment (eg. model deployment, function deployment)
    
    Attributes:
        inference_global (Union[Unset, list['Metric']]): Array of metrics
        last_n_requests (Union[Unset, list['Metric']]): Array of metrics
        latency (Union[Unset, LatencyMetric]): Latency metrics
        request_duration_over_time (Union[Unset, RequestDurationOverTimeMetrics]): Request duration over time metrics
        request_total (Union[Unset, float]): Number of requests for the resource globally
        request_total_by_origin (Union[Unset, RequestTotalByOriginMetric]): Request total by origin metric
        request_total_per_code (Union[Unset, ResourceMetricsRequestTotalPerCode]): Number of requests for the resource
            globally per code
        rps (Union[Unset, float]): Number of requests per second for the resource globally
        rps_per_code (Union[Unset, ResourceMetricsRpsPerCode]): Number of requests per second for the resource globally
            per code
        token_rate (Union[Unset, TokenRateMetrics]): Token rate metrics
        token_total (Union[Unset, TokenTotalMetric]): Token total metric
    
    Method generated by attrs for class ResourceMetrics.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties`
    :

    `inference_global`
    :

    `last_n_requests`
    :

    `latency`
    :

    `request_duration_over_time`
    :

    `request_total`
    :

    `request_total_by_origin`
    :

    `request_total_per_code`
    :

    `rps`
    :

    `rps_per_code`
    :

    `token_rate`
    :

    `token_total`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`ResourceMetricsRequestTotalPerCode()`
:   Number of requests for the resource globally per code
    
    Method generated by attrs for class ResourceMetricsRequestTotalPerCode.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`ResourceMetricsRpsPerCode()`
:   Number of requests per second for the resource globally per code
    
    Method generated by attrs for class ResourceMetricsRpsPerCode.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`RevisionMetadata(created_at: beamlit.types.Unset | str = <beamlit.types.Unset object>, id: beamlit.types.Unset | str = <beamlit.types.Unset object>)`
:   Revision metadata
    
    Attributes:
        created_at (Union[Unset, str]): Revision created at
        id (Union[Unset, str]): Revision ID
    
    Method generated by attrs for class RevisionMetadata.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    `created_at: beamlit.types.Unset | str`
    :

    `id: beamlit.types.Unset | str`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`Runtime(args: beamlit.types.Unset | list[typing.Any] = <beamlit.types.Unset object>, command: beamlit.types.Unset | list[typing.Any] = <beamlit.types.Unset object>, endpoint_name: beamlit.types.Unset | str = <beamlit.types.Unset object>, envs: beamlit.types.Unset | list[typing.Any] = <beamlit.types.Unset object>, image: beamlit.types.Unset | str = <beamlit.types.Unset object>, metric_port: beamlit.types.Unset | int = <beamlit.types.Unset object>, model: beamlit.types.Unset | str = <beamlit.types.Unset object>, organization: beamlit.types.Unset | str = <beamlit.types.Unset object>, readiness_probe: beamlit.types.Unset | ForwardRef('RuntimeReadinessProbe') = <beamlit.types.Unset object>, resources: beamlit.types.Unset | ForwardRef('RuntimeResources') = <beamlit.types.Unset object>, serving_port: beamlit.types.Unset | int = <beamlit.types.Unset object>, type_: beamlit.types.Unset | str = <beamlit.types.Unset object>)`
:   Set of configurations for a deployment
    
    Attributes:
        args (Union[Unset, list[Any]]): The arguments to pass to the deployment runtime
        command (Union[Unset, list[Any]]): The command to run the deployment
        endpoint_name (Union[Unset, str]): Endpoint Name of the model. In case of hf_private_endpoint, it is the
            endpoint name. In case of hf_public_endpoint, it is not used.
        envs (Union[Unset, list[Any]]): The env variables to set in the deployment. Should be a list of Kubernetes
            EnvVar types
        image (Union[Unset, str]): The Docker image for the deployment
        metric_port (Union[Unset, int]): The port to serve the metrics on
        model (Union[Unset, str]): The slug name of the origin model at HuggingFace.
        organization (Union[Unset, str]): The organization of the model
        readiness_probe (Union[Unset, RuntimeReadinessProbe]): The readiness probe. Should be a Kubernetes Probe type
        resources (Union[Unset, RuntimeResources]): The resources for the deployment. Should be a Kubernetes
            ResourceRequirements type
        serving_port (Union[Unset, int]): The port to serve the model on
        type_ (Union[Unset, str]): The type of origin for the deployment (hf_private_endpoint, hf_public_endpoint)
    
    Method generated by attrs for class Runtime.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties`
    :

    `args`
    :

    `command`
    :

    `endpoint_name`
    :

    `envs`
    :

    `image`
    :

    `metric_port`
    :

    `model`
    :

    `organization`
    :

    `readiness_probe`
    :

    `resources`
    :

    `serving_port`
    :

    `type_`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`RuntimeReadinessProbe()`
:   The readiness probe. Should be a Kubernetes Probe type
    
    Method generated by attrs for class RuntimeReadinessProbe.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`RuntimeResources()`
:   The resources for the deployment. Should be a Kubernetes ResourceRequirements type
    
    Method generated by attrs for class RuntimeResources.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`ServerlessConfig(last_pod_retention_period: beamlit.types.Unset | str = <beamlit.types.Unset object>, max_num_replicas: beamlit.types.Unset | int = <beamlit.types.Unset object>, metric: beamlit.types.Unset | str = <beamlit.types.Unset object>, min_num_replicas: beamlit.types.Unset | int = <beamlit.types.Unset object>, scale_down_delay: beamlit.types.Unset | str = <beamlit.types.Unset object>, scale_up_minimum: beamlit.types.Unset | int = <beamlit.types.Unset object>, stable_window: beamlit.types.Unset | str = <beamlit.types.Unset object>, target: beamlit.types.Unset | str = <beamlit.types.Unset object>)`
:   Configuration for a serverless deployment
    
    Attributes:
        last_pod_retention_period (Union[Unset, str]): The minimum amount of time that the last replica will remain
            active AFTER a scale-to-zero decision is made
        max_num_replicas (Union[Unset, int]): The maximum number of replicas for the deployment.
        metric (Union[Unset, str]): Metric watched to make scaling decisions. Can be "cpu" or "memory" or "rps" or
            "concurrency"
        min_num_replicas (Union[Unset, int]): The minimum number of replicas for the deployment. Can be 0 or 1 (in which
            case the deployment is always running in at least one location).
        scale_down_delay (Union[Unset, str]): The time window which must pass at reduced concurrency before a scale-down
            decision is applied. This can be useful, for example, to keep containers around for a configurable duration to
            avoid a cold start penalty if new requests come in.
        scale_up_minimum (Union[Unset, int]): The minimum number of replicas that will be created when the deployment
            scales up from zero.
        stable_window (Union[Unset, str]): The sliding time window over which metrics are averaged to provide the input
            for scaling decisions
        target (Union[Unset, str]): Target value for the watched metric
    
    Method generated by attrs for class ServerlessConfig.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    `last_pod_retention_period: beamlit.types.Unset | str`
    :

    `max_num_replicas: beamlit.types.Unset | int`
    :

    `metric: beamlit.types.Unset | str`
    :

    `min_num_replicas: beamlit.types.Unset | int`
    :

    `scale_down_delay: beamlit.types.Unset | str`
    :

    `scale_up_minimum: beamlit.types.Unset | int`
    :

    `stable_window: beamlit.types.Unset | str`
    :

    `target: beamlit.types.Unset | str`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`SpecConfiguration(secret: beamlit.types.Unset | bool = <beamlit.types.Unset object>, value: beamlit.types.Unset | str = <beamlit.types.Unset object>)`
:   Configuration, this is a key value storage. In your object you can retrieve the value with config[key]
    
    Attributes:
        secret (Union[Unset, bool]): ACconfiguration secret
        value (Union[Unset, str]): Configuration value
    
    Method generated by attrs for class SpecConfiguration.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    `secret: beamlit.types.Unset | bool`
    :

    `value: beamlit.types.Unset | str`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`StoreAgent(created_at: beamlit.types.Unset | str = <beamlit.types.Unset object>, updated_at: beamlit.types.Unset | str = <beamlit.types.Unset object>, created_by: beamlit.types.Unset | str = <beamlit.types.Unset object>, updated_by: beamlit.types.Unset | str = <beamlit.types.Unset object>, configuration: beamlit.types.Unset | list['StoreConfiguration'] = <beamlit.types.Unset object>, description: beamlit.types.Unset | str = <beamlit.types.Unset object>, display_name: beamlit.types.Unset | str = <beamlit.types.Unset object>, image: beamlit.types.Unset | str = <beamlit.types.Unset object>, labels: beamlit.types.Unset | ForwardRef('StoreAgentLabels') = <beamlit.types.Unset object>, name: beamlit.types.Unset | str = <beamlit.types.Unset object>, prompt: beamlit.types.Unset | str = <beamlit.types.Unset object>)`
:   Store agent
    
    Attributes:
        created_at (Union[Unset, str]): The date and time when the resource was created
        updated_at (Union[Unset, str]): The date and time when the resource was updated
        created_by (Union[Unset, str]): The user or service account who created the resource
        updated_by (Union[Unset, str]): The user or service account who updated the resource
        configuration (Union[Unset, list['StoreConfiguration']]): Store agent configuration
        description (Union[Unset, str]): Store agent description
        display_name (Union[Unset, str]): Store agent display name
        image (Union[Unset, str]): Store agent image
        labels (Union[Unset, StoreAgentLabels]): Store agent labels
        name (Union[Unset, str]): Store agent name
        prompt (Union[Unset, str]): Store agent prompt, this is to define what the agent does
    
    Method generated by attrs for class StoreAgent.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties`
    :

    `configuration`
    :

    `created_at`
    :

    `created_by`
    :

    `description`
    :

    `display_name`
    :

    `image`
    :

    `labels`
    :

    `name`
    :

    `prompt`
    :

    `updated_at`
    :

    `updated_by`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`StoreAgentLabels()`
:   Store agent labels
    
    Method generated by attrs for class StoreAgentLabels.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`StoreConfiguration(available_models: beamlit.types.Unset | list[str] = <beamlit.types.Unset object>, description: beamlit.types.Unset | str = <beamlit.types.Unset object>, display_name: beamlit.types.Unset | str = <beamlit.types.Unset object>, if_: beamlit.types.Unset | str = <beamlit.types.Unset object>, name: beamlit.types.Unset | str = <beamlit.types.Unset object>, options: beamlit.types.Unset | list['StoreConfigurationOption'] = <beamlit.types.Unset object>, required: beamlit.types.Unset | bool = <beamlit.types.Unset object>, secret: beamlit.types.Unset | bool = <beamlit.types.Unset object>, type_: beamlit.types.Unset | str = <beamlit.types.Unset object>)`
:   Store configuration for resources (eg: agent, function, etc)
    
    Attributes:
        available_models (Union[Unset, list[str]]): Available models for the configuration
        description (Union[Unset, str]): Store configuration description
        display_name (Union[Unset, str]): Store configuration display name
        if_ (Union[Unset, str]): Conditional rendering for the configuration, example: provider === 'openai'
        name (Union[Unset, str]): Store configuration name
        options (Union[Unset, list['StoreConfigurationOption']]):
        required (Union[Unset, bool]): Store configuration required
        secret (Union[Unset, bool]): Store configuration secret
        type_ (Union[Unset, str]): Store configuration type
    
    Method generated by attrs for class StoreConfiguration.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties`
    :

    `available_models`
    :

    `description`
    :

    `display_name`
    :

    `if_`
    :

    `name`
    :

    `options`
    :

    `required`
    :

    `secret`
    :

    `type_`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`StoreConfigurationOption(if_: beamlit.types.Unset | str = <beamlit.types.Unset object>, label: beamlit.types.Unset | str = <beamlit.types.Unset object>, value: beamlit.types.Unset | str = <beamlit.types.Unset object>)`
:   Store configuration options for a select type configuration
    
    Attributes:
        if_ (Union[Unset, str]): Conditional rendering for the configuration option, example: provider === 'openai'
        label (Union[Unset, str]): Store configuration option label
        value (Union[Unset, str]): Store configuration option value
    
    Method generated by attrs for class StoreConfigurationOption.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    `if_: beamlit.types.Unset | str`
    :

    `label: beamlit.types.Unset | str`
    :

    `value: beamlit.types.Unset | str`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`StoreFunction(created_at: beamlit.types.Unset | str = <beamlit.types.Unset object>, updated_at: beamlit.types.Unset | str = <beamlit.types.Unset object>, created_by: beamlit.types.Unset | str = <beamlit.types.Unset object>, updated_by: beamlit.types.Unset | str = <beamlit.types.Unset object>, configuration: beamlit.types.Unset | list['StoreConfiguration'] = <beamlit.types.Unset object>, description: beamlit.types.Unset | str = <beamlit.types.Unset object>, display_name: beamlit.types.Unset | str = <beamlit.types.Unset object>, image: beamlit.types.Unset | str = <beamlit.types.Unset object>, kit: beamlit.types.Unset | list['StoreFunctionKit'] = <beamlit.types.Unset object>, labels: beamlit.types.Unset | ForwardRef('StoreFunctionLabels') = <beamlit.types.Unset object>, name: beamlit.types.Unset | str = <beamlit.types.Unset object>, parameters: beamlit.types.Unset | list['StoreFunctionParameter'] = <beamlit.types.Unset object>)`
:   Store function
    
    Attributes:
        created_at (Union[Unset, str]): The date and time when the resource was created
        updated_at (Union[Unset, str]): The date and time when the resource was updated
        created_by (Union[Unset, str]): The user or service account who created the resource
        updated_by (Union[Unset, str]): The user or service account who updated the resource
        configuration (Union[Unset, list['StoreConfiguration']]): Store function configuration
        description (Union[Unset, str]): Store function description
        display_name (Union[Unset, str]): Store function display name
        image (Union[Unset, str]): Store function image
        kit (Union[Unset, list['StoreFunctionKit']]): Store function kit
        labels (Union[Unset, StoreFunctionLabels]): Store function labels
        name (Union[Unset, str]): Store function name
        parameters (Union[Unset, list['StoreFunctionParameter']]): Store function parameters
    
    Method generated by attrs for class StoreFunction.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties`
    :

    `configuration`
    :

    `created_at`
    :

    `created_by`
    :

    `description`
    :

    `display_name`
    :

    `image`
    :

    `kit`
    :

    `labels`
    :

    `name`
    :

    `parameters`
    :

    `updated_at`
    :

    `updated_by`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`StoreFunctionKit(description: beamlit.types.Unset | str = <beamlit.types.Unset object>, name: beamlit.types.Unset | str = <beamlit.types.Unset object>, parameters: beamlit.types.Unset | list['StoreFunctionParameter'] = <beamlit.types.Unset object>)`
:   Store function kit
    
    Attributes:
        description (Union[Unset, str]): Description of the function kit, very important for the agent to work with your
            kit
        name (Union[Unset, str]): The kit name, very important for the agent to work with your kit
        parameters (Union[Unset, list['StoreFunctionParameter']]): Kit parameters, for your kit to be callable with an
            Agent
    
    Method generated by attrs for class StoreFunctionKit.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties`
    :

    `description`
    :

    `name`
    :

    `parameters`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`StoreFunctionLabels()`
:   Store function labels
    
    Method generated by attrs for class StoreFunctionLabels.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`StoreFunctionParameter(description: beamlit.types.Unset | str = <beamlit.types.Unset object>, name: beamlit.types.Unset | str = <beamlit.types.Unset object>, required: beamlit.types.Unset | bool = <beamlit.types.Unset object>, type_: beamlit.types.Unset | str = <beamlit.types.Unset object>)`
:   Store function parameter
    
    Attributes:
        description (Union[Unset, str]): Store function parameter description
        name (Union[Unset, str]): Store function parameter name
        required (Union[Unset, bool]): Store function parameter required
        type_ (Union[Unset, str]): Store function parameter type
    
    Method generated by attrs for class StoreFunctionParameter.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    `description: beamlit.types.Unset | str`
    :

    `name: beamlit.types.Unset | str`
    :

    `required: beamlit.types.Unset | bool`
    :

    `type_: beamlit.types.Unset | str`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`TimeFields(created_at: beamlit.types.Unset | str = <beamlit.types.Unset object>, updated_at: beamlit.types.Unset | str = <beamlit.types.Unset object>)`
:   Time fields for Persistance
    
    Attributes:
        created_at (Union[Unset, str]): The date and time when the resource was created
        updated_at (Union[Unset, str]): The date and time when the resource was updated
    
    Method generated by attrs for class TimeFields.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    `created_at: beamlit.types.Unset | str`
    :

    `updated_at: beamlit.types.Unset | str`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`TokenRateMetric(model: beamlit.types.Unset | str = <beamlit.types.Unset object>, timestamp: beamlit.types.Unset | str = <beamlit.types.Unset object>, token_total: beamlit.types.Unset | float = <beamlit.types.Unset object>, trend: beamlit.types.Unset | float = <beamlit.types.Unset object>)`
:   Token rate metric
    
    Attributes:
        model (Union[Unset, str]): Model ID
        timestamp (Union[Unset, str]): Timestamp
        token_total (Union[Unset, float]): Total tokens
        trend (Union[Unset, float]): Trend
    
    Method generated by attrs for class TokenRateMetric.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    `model: beamlit.types.Unset | str`
    :

    `timestamp: beamlit.types.Unset | str`
    :

    `token_total: beamlit.types.Unset | float`
    :

    `trend: beamlit.types.Unset | float`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`TokenRateMetrics(token_rate: beamlit.types.Unset | ForwardRef('TokenRateMetric') = <beamlit.types.Unset object>, token_rate_input: beamlit.types.Unset | ForwardRef('TokenRateMetric') = <beamlit.types.Unset object>, token_rate_output: beamlit.types.Unset | ForwardRef('TokenRateMetric') = <beamlit.types.Unset object>)`
:   Token rate metrics
    
    Attributes:
        token_rate (Union[Unset, TokenRateMetric]): Token rate metric
        token_rate_input (Union[Unset, TokenRateMetric]): Token rate metric
        token_rate_output (Union[Unset, TokenRateMetric]): Token rate metric
    
    Method generated by attrs for class TokenRateMetrics.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties`
    :

    `token_rate`
    :

    `token_rate_input`
    :

    `token_rate_output`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`TokenTotalMetric(average_token_input_per_request: beamlit.types.Unset | float = <beamlit.types.Unset object>, average_token_output_per_request: beamlit.types.Unset | float = <beamlit.types.Unset object>, average_token_per_request: beamlit.types.Unset | float = <beamlit.types.Unset object>, token_input: beamlit.types.Unset | float = <beamlit.types.Unset object>, token_output: beamlit.types.Unset | float = <beamlit.types.Unset object>, token_total: beamlit.types.Unset | float = <beamlit.types.Unset object>)`
:   Token total metric
    
    Attributes:
        average_token_input_per_request (Union[Unset, float]): Average input token per request
        average_token_output_per_request (Union[Unset, float]): Average output token per request
        average_token_per_request (Union[Unset, float]): Average token per request
        token_input (Union[Unset, float]): Total input tokens
        token_output (Union[Unset, float]): Total output tokens
        token_total (Union[Unset, float]): Total tokens
    
    Method generated by attrs for class TokenTotalMetric.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    `average_token_input_per_request: beamlit.types.Unset | float`
    :

    `average_token_output_per_request: beamlit.types.Unset | float`
    :

    `average_token_per_request: beamlit.types.Unset | float`
    :

    `token_input: beamlit.types.Unset | float`
    :

    `token_output: beamlit.types.Unset | float`
    :

    `token_total: beamlit.types.Unset | float`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`TraceIdsResponse()`
:   Trace IDs response
    
    Method generated by attrs for class TraceIdsResponse.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`UpdateWorkspaceServiceAccountBody(description: beamlit.types.Unset | str = <beamlit.types.Unset object>, name: beamlit.types.Unset | str = <beamlit.types.Unset object>)`
:   Attributes:
        description (Union[Unset, str]): Service account description
        name (Union[Unset, str]): Service account name
    
    Method generated by attrs for class UpdateWorkspaceServiceAccountBody.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    `description: beamlit.types.Unset | str`
    :

    `name: beamlit.types.Unset | str`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`UpdateWorkspaceServiceAccountResponse200(client_id: beamlit.types.Unset | str = <beamlit.types.Unset object>, created_at: beamlit.types.Unset | str = <beamlit.types.Unset object>, description: beamlit.types.Unset | str = <beamlit.types.Unset object>, name: beamlit.types.Unset | str = <beamlit.types.Unset object>, updated_at: beamlit.types.Unset | str = <beamlit.types.Unset object>)`
:   Attributes:
        client_id (Union[Unset, str]): Service account client ID
        created_at (Union[Unset, str]): Creation timestamp
        description (Union[Unset, str]): Service account description
        name (Union[Unset, str]): Service account name
        updated_at (Union[Unset, str]): Last update timestamp
    
    Method generated by attrs for class UpdateWorkspaceServiceAccountResponse200.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    `client_id: beamlit.types.Unset | str`
    :

    `created_at: beamlit.types.Unset | str`
    :

    `description: beamlit.types.Unset | str`
    :

    `name: beamlit.types.Unset | str`
    :

    `updated_at: beamlit.types.Unset | str`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`UpdateWorkspaceUserRoleBody(role: str)`
:   Attributes:
        role (str): The new role to assign to the user
    
    Method generated by attrs for class UpdateWorkspaceUserRoleBody.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    `role: str`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`WebsocketChannel(created_at: beamlit.types.Unset | str = <beamlit.types.Unset object>, updated_at: beamlit.types.Unset | str = <beamlit.types.Unset object>, connection_id: beamlit.types.Unset | str = <beamlit.types.Unset object>, workspace: beamlit.types.Unset | str = <beamlit.types.Unset object>)`
:   WebSocket connection details
    
    Attributes:
        created_at (Union[Unset, str]): The date and time when the resource was created
        updated_at (Union[Unset, str]): The date and time when the resource was updated
        connection_id (Union[Unset, str]): Unique connection ID
        workspace (Union[Unset, str]): Workspace the connection belongs to
    
    Method generated by attrs for class WebsocketChannel.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    `connection_id: beamlit.types.Unset | str`
    :

    `created_at: beamlit.types.Unset | str`
    :

    `updated_at: beamlit.types.Unset | str`
    :

    `workspace: beamlit.types.Unset | str`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`Workspace(created_at: beamlit.types.Unset | str = <beamlit.types.Unset object>, updated_at: beamlit.types.Unset | str = <beamlit.types.Unset object>, created_by: beamlit.types.Unset | str = <beamlit.types.Unset object>, updated_by: beamlit.types.Unset | str = <beamlit.types.Unset object>, account_id: beamlit.types.Unset | str = <beamlit.types.Unset object>, display_name: beamlit.types.Unset | str = <beamlit.types.Unset object>, labels: beamlit.types.Unset | ForwardRef('WorkspaceLabels') = <beamlit.types.Unset object>, name: beamlit.types.Unset | str = <beamlit.types.Unset object>, region: beamlit.types.Unset | str = <beamlit.types.Unset object>)`
:   Workspace
    
    Attributes:
        created_at (Union[Unset, str]): The date and time when the resource was created
        updated_at (Union[Unset, str]): The date and time when the resource was updated
        created_by (Union[Unset, str]): The user or service account who created the resource
        updated_by (Union[Unset, str]): The user or service account who updated the resource
        account_id (Union[Unset, str]): Workspace account id
        display_name (Union[Unset, str]): Workspace display name
        labels (Union[Unset, WorkspaceLabels]): Workspace labels
        name (Union[Unset, str]): Workspace name
        region (Union[Unset, str]): Workspace write region
    
    Method generated by attrs for class Workspace.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `account_id`
    :

    `additional_keys: list[str]`
    :

    `additional_properties`
    :

    `created_at`
    :

    `created_by`
    :

    `display_name`
    :

    `labels`
    :

    `name`
    :

    `region`
    :

    `updated_at`
    :

    `updated_by`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`WorkspaceLabels()`
:   Workspace labels
    
    Method generated by attrs for class WorkspaceLabels.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :

`WorkspaceUser(accepted: beamlit.types.Unset | bool = <beamlit.types.Unset object>, email: beamlit.types.Unset | str = <beamlit.types.Unset object>, email_verified: beamlit.types.Unset | bool = <beamlit.types.Unset object>, family_name: beamlit.types.Unset | str = <beamlit.types.Unset object>, given_name: beamlit.types.Unset | str = <beamlit.types.Unset object>, role: beamlit.types.Unset | str = <beamlit.types.Unset object>, sub: beamlit.types.Unset | str = <beamlit.types.Unset object>)`
:   Workspace user
    
    Attributes:
        accepted (Union[Unset, bool]): Whether the user has accepted the workspace invitation
        email (Union[Unset, str]): Workspace user email
        email_verified (Union[Unset, bool]): Whether the user's email has been verified
        family_name (Union[Unset, str]): Workspace user family name
        given_name (Union[Unset, str]): Workspace user given name
        role (Union[Unset, str]): Workspace user role
        sub (Union[Unset, str]): Workspace user identifier
    
    Method generated by attrs for class WorkspaceUser.

    ### Static methods

    `from_dict(src_dict: dict[str, typing.Any]) ‑> ~T`
    :

    ### Instance variables

    `accepted: beamlit.types.Unset | bool`
    :

    `additional_keys: list[str]`
    :

    `additional_properties: dict[str, typing.Any]`
    :

    `email: beamlit.types.Unset | str`
    :

    `email_verified: beamlit.types.Unset | bool`
    :

    `family_name: beamlit.types.Unset | str`
    :

    `given_name: beamlit.types.Unset | str`
    :

    `role: beamlit.types.Unset | str`
    :

    `sub: beamlit.types.Unset | str`
    :

    ### Methods

    `to_dict(self) ‑> dict[str, typing.Any]`
    :