"""
This file was generated by the KCL auto-gen tool. DO NOT EDIT.
Editing this file might prove futile when you re-run the KCL auto-gen generate command.
"""

import regex
import k8s.apimachinery.pkg.apis.meta.v1

_regex_match = regex.match

schema ExternalSecret:
    r"""
    ExternalSecret is the Schema for the external-secrets API.

    Attributes
    ----------
    apiVersion : str, default is "external-secrets.io/v1beta1", required
        APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources
    kind : str, default is "ExternalSecret", required
        Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds
    metadata : v1.ObjectMeta, default is Undefined, optional
        metadata
    spec : ExternalSecretsIoV1beta1ExternalSecretSpec, default is Undefined, optional
        spec
    status : ExternalSecretsIoV1beta1ExternalSecretStatus, default is Undefined, optional
        status
    """
    apiVersion: "external-secrets.io/v1beta1" = "external-secrets.io/v1beta1"

    kind: "ExternalSecret" = "ExternalSecret"

    metadata?: v1.ObjectMeta
    spec?: ExternalSecretsIoV1beta1ExternalSecretSpec
    status?: ExternalSecretsIoV1beta1ExternalSecretStatus

schema ExternalSecretsIoV1beta1ExternalSecretSpec:
    r"""
    ExternalSecretSpec defines the desired state of ExternalSecret.

    Attributes
    ----------
    data : [ExternalSecretsIoV1beta1ExternalSecretSpecDataItems0], default is Undefined, optional
        Data defines the connection between the Kubernetes Secret keys and the Provider data
    dataFrom : [ExternalSecretsIoV1beta1ExternalSecretSpecDataFromItems0], default is Undefined, optional
        DataFrom is used to fetch all properties from a specific Provider data
        If multiple entries are specified, the Secret keys are merged in the specified order
    refreshInterval : str, default is "1h", optional
        RefreshInterval is the amount of time before the values are read again from the SecretStore provider,
        specified as Golang Duration strings.
        Valid time units are "ns", "us" (or "µs"), "ms", "s", "m", "h"
        Example values: "1h", "2h30m", "10s"
        May be set to zero to fetch and create it once. Defaults to 1h.
    refreshPolicy : str, default is Undefined, optional
        RefreshPolicy determines how the ExternalSecret should be refreshed:
        - CreatedOnce: Creates the Secret only if it does not exist and does not update it thereafter
        - Periodic: Synchronizes the Secret from the external source at regular intervals specified by refreshInterval.
          No periodic updates occur if refreshInterval is 0.
        - OnChange: Only synchronizes the Secret when the ExternalSecret's metadata or specification changes
    secretStoreRef : ExternalSecretsIoV1beta1ExternalSecretSpecSecretStoreRef, default is Undefined, optional
        secret store ref
    target : ExternalSecretsIoV1beta1ExternalSecretSpecTarget, default is Undefined, optional
        target
    """
    data?: [ExternalSecretsIoV1beta1ExternalSecretSpecDataItems0]
    dataFrom?: [ExternalSecretsIoV1beta1ExternalSecretSpecDataFromItems0]
    refreshInterval?: str = "1h"
    refreshPolicy?: "CreatedOnce" | "Periodic" | "OnChange"
    secretStoreRef?: ExternalSecretsIoV1beta1ExternalSecretSpecSecretStoreRef
    target?: ExternalSecretsIoV1beta1ExternalSecretSpecTarget

schema ExternalSecretsIoV1beta1ExternalSecretSpecDataFromItems0:
    r"""
    external secrets io v1beta1 external secret spec data from items0

    Attributes
    ----------
    extract : ExternalSecretsIoV1beta1ExternalSecretSpecDataFromItems0Extract, default is Undefined, optional
        extract
    find : ExternalSecretsIoV1beta1ExternalSecretSpecDataFromItems0Find, default is Undefined, optional
        find
    rewrite : [ExternalSecretsIoV1beta1ExternalSecretSpecDataFromItems0RewriteItems0], default is Undefined, optional
        Used to rewrite secret Keys after getting them from the secret Provider
        Multiple Rewrite operations can be provided. They are applied in a layered order (first to last)
    sourceRef : ExternalSecretsIoV1beta1ExternalSecretSpecDataFromItems0SourceRef, default is Undefined, optional
        source ref
    """
    extract?: ExternalSecretsIoV1beta1ExternalSecretSpecDataFromItems0Extract
    find?: ExternalSecretsIoV1beta1ExternalSecretSpecDataFromItems0Find
    rewrite?: [ExternalSecretsIoV1beta1ExternalSecretSpecDataFromItems0RewriteItems0]
    sourceRef?: ExternalSecretsIoV1beta1ExternalSecretSpecDataFromItems0SourceRef

schema ExternalSecretsIoV1beta1ExternalSecretSpecDataFromItems0Extract:
    r"""
    Used to extract multiple key/value pairs from one secret
    Note: Extract does not support sourceRef.Generator or sourceRef.GeneratorRef.

    Attributes
    ----------
    conversionStrategy : str, default is "Default", optional
        Used to define a conversion Strategy
    decodingStrategy : str, default is "None", optional
        Used to define a decoding Strategy
    key : str, default is Undefined, required
        Key is the key used in the Provider, mandatory
    metadataPolicy : str, default is "None", optional
        Policy for fetching tags/labels from provider secrets, possible options are Fetch, None. Defaults to None
    property : str, default is Undefined, optional
        Used to select a specific property of the Provider value (if a map), if supported
    version : str, default is Undefined, optional
        Used to select a specific version of the Provider value, if supported
    """
    conversionStrategy?: "Default" | "Unicode" = "Default"
    decodingStrategy?: "Auto" | "Base64" | "Base64URL" | "None" = "None"
    key: str

    metadataPolicy?: "None" | "Fetch" = "None"
    property?: str
    version?: str

schema ExternalSecretsIoV1beta1ExternalSecretSpecDataFromItems0Find:
    r"""
    Used to find secrets based on tags or regular expressions
    Note: Find does not support sourceRef.Generator or sourceRef.GeneratorRef.

    Attributes
    ----------
    conversionStrategy : str, default is "Default", optional
        Used to define a conversion Strategy
    decodingStrategy : str, default is "None", optional
        Used to define a decoding Strategy
    name : ExternalSecretsIoV1beta1ExternalSecretSpecDataFromItems0FindName, default is Undefined, optional
        name
    path : str, default is Undefined, optional
        A root path to start the find operations.
    tags : {str:str}, default is Undefined, optional
        Find secrets based on tags.
    """
    conversionStrategy?: "Default" | "Unicode" = "Default"
    decodingStrategy?: "Auto" | "Base64" | "Base64URL" | "None" = "None"
    name?: ExternalSecretsIoV1beta1ExternalSecretSpecDataFromItems0FindName
    path?: str
    tags?: {str:str}

schema ExternalSecretsIoV1beta1ExternalSecretSpecDataFromItems0FindName:
    r"""
    Finds secrets based on the name.

    Attributes
    ----------
    regexp : str, default is Undefined, optional
        Finds secrets base
    """
    regexp?: str

schema ExternalSecretsIoV1beta1ExternalSecretSpecDataFromItems0RewriteItems0:
    r"""
    external secrets io v1beta1 external secret spec data from items0 rewrite items0

    Attributes
    ----------
    regexp : ExternalSecretsIoV1beta1ExternalSecretSpecDataFromItems0RewriteItems0Regexp, default is Undefined, optional
        regexp
    transform : ExternalSecretsIoV1beta1ExternalSecretSpecDataFromItems0RewriteItems0Transform, default is Undefined, optional
        transform
    """
    regexp?: ExternalSecretsIoV1beta1ExternalSecretSpecDataFromItems0RewriteItems0Regexp
    transform?: ExternalSecretsIoV1beta1ExternalSecretSpecDataFromItems0RewriteItems0Transform

schema ExternalSecretsIoV1beta1ExternalSecretSpecDataFromItems0RewriteItems0Regexp:
    r"""
    Used to rewrite with regular expressions.
    The resulting key will be the output of a regexp.ReplaceAll operation.

    Attributes
    ----------
    source : str, default is Undefined, required
        Used to define the regular expression of a re.Compiler.
    target : str, default is Undefined, required
        Used to define the target pattern of a ReplaceAll operation.
    """
    source: str

    target: str

schema ExternalSecretsIoV1beta1ExternalSecretSpecDataFromItems0RewriteItems0Transform:
    r"""
    Used to apply string transformation on the secrets.
    The resulting key will be the output of the template applied by the operation.

    Attributes
    ----------
    template : str, default is Undefined, required
        Used to define the template to apply on the secret name.
        `.value ` will specify the secret name in the template.
    """
    template: str

schema ExternalSecretsIoV1beta1ExternalSecretSpecDataFromItems0SourceRef:
    r"""
    SourceRef points to a store or generator
    which contains secret values ready to use.
    Use this in combination with Extract or Find pull values out of
    a specific SecretStore.
    When sourceRef points to a generator Extract or Find is not supported.
    The generator returns a static map of values

    Attributes
    ----------
    generatorRef : ExternalSecretsIoV1beta1ExternalSecretSpecDataFromItems0SourceRefGeneratorRef, default is Undefined, optional
        generator ref
    storeRef : ExternalSecretsIoV1beta1ExternalSecretSpecDataFromItems0SourceRefStoreRef, default is Undefined, optional
        store ref
    """
    generatorRef?: ExternalSecretsIoV1beta1ExternalSecretSpecDataFromItems0SourceRefGeneratorRef
    storeRef?: ExternalSecretsIoV1beta1ExternalSecretSpecDataFromItems0SourceRefStoreRef

schema ExternalSecretsIoV1beta1ExternalSecretSpecDataFromItems0SourceRefGeneratorRef:
    r"""
    GeneratorRef points to a generator custom resource.

    Attributes
    ----------
    apiVersion : str, default is "generators.external-secrets.io/v1alpha1", optional
        Specify the apiVersion of the generator resource
    kind : str, default is Undefined, required
        Specify the Kind of the generator resource
    name : str, default is Undefined, required
        Specify the name of the generator resource
    """
    apiVersion?: str = "generators.external-secrets.io/v1alpha1"
    kind: "ACRAccessToken" | "ClusterGenerator" | "ECRAuthorizationToken" | "Fake" | "GCRAccessToken" | "GithubAccessToken" | "QuayAccessToken" | "Password" | "STSSessionToken" | "UUID" | "VaultDynamicSecret" | "Webhook" | "Grafana"

    name: str

    check:
        len(name) <= 253
        len(name) >= 1
        _regex_match(str(name), r"^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$")

schema ExternalSecretsIoV1beta1ExternalSecretSpecDataFromItems0SourceRefStoreRef:
    r"""
    SecretStoreRef defines which SecretStore to fetch the ExternalSecret data.

    Attributes
    ----------
    kind : str, default is Undefined, optional
        Kind of the SecretStore resource (SecretStore or ClusterSecretStore)
        Defaults to `SecretStore`
    name : str, default is Undefined, optional
        Name of the SecretStore resource
    """
    kind?: "SecretStore" | "ClusterSecretStore"
    name?: str

    check:
        len(name) <= 253 if name
        len(name) >= 1 if name
        _regex_match(str(name), r"^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$") if name

schema ExternalSecretsIoV1beta1ExternalSecretSpecDataItems0:
    r"""
    ExternalSecretData defines the connection between the Kubernetes Secret key (spec.data.<key>) and the Provider data.

    Attributes
    ----------
    remoteRef : ExternalSecretsIoV1beta1ExternalSecretSpecDataItems0RemoteRef, default is Undefined, required
        remote ref
    secretKey : str, default is Undefined, required
        The key in the Kubernetes Secret to store the value.
    sourceRef : ExternalSecretsIoV1beta1ExternalSecretSpecDataItems0SourceRef, default is Undefined, optional
        source ref
    """
    remoteRef: ExternalSecretsIoV1beta1ExternalSecretSpecDataItems0RemoteRef

    secretKey: str

    sourceRef?: ExternalSecretsIoV1beta1ExternalSecretSpecDataItems0SourceRef

    check:
        len(secretKey) <= 253
        len(secretKey) >= 1
        _regex_match(str(secretKey), r"^[-._a-zA-Z0-9]+$")

schema ExternalSecretsIoV1beta1ExternalSecretSpecDataItems0RemoteRef:
    r"""
    RemoteRef points to the remote secret and defines
    which secret (version/property/..) to fetch.

    Attributes
    ----------
    conversionStrategy : str, default is "Default", optional
        Used to define a conversion Strategy
    decodingStrategy : str, default is "None", optional
        Used to define a decoding Strategy
    key : str, default is Undefined, required
        Key is the key used in the Provider, mandatory
    metadataPolicy : str, default is "None", optional
        Policy for fetching tags/labels from provider secrets, possible options are Fetch, None. Defaults to None
    property : str, default is Undefined, optional
        Used to select a specific property of the Provider value (if a map), if supported
    version : str, default is Undefined, optional
        Used to select a specific version of the Provider value, if supported
    """
    conversionStrategy?: "Default" | "Unicode" = "Default"
    decodingStrategy?: "Auto" | "Base64" | "Base64URL" | "None" = "None"
    key: str

    metadataPolicy?: "None" | "Fetch" = "None"
    property?: str
    version?: str

schema ExternalSecretsIoV1beta1ExternalSecretSpecDataItems0SourceRef:
    r"""
    SourceRef allows you to override the source
    from which the value will be pulled.

    Attributes
    ----------
    generatorRef : ExternalSecretsIoV1beta1ExternalSecretSpecDataItems0SourceRefGeneratorRef, default is Undefined, optional
        generator ref
    storeRef : ExternalSecretsIoV1beta1ExternalSecretSpecDataItems0SourceRefStoreRef, default is Undefined, optional
        store ref
    """
    generatorRef?: ExternalSecretsIoV1beta1ExternalSecretSpecDataItems0SourceRefGeneratorRef
    storeRef?: ExternalSecretsIoV1beta1ExternalSecretSpecDataItems0SourceRefStoreRef

schema ExternalSecretsIoV1beta1ExternalSecretSpecDataItems0SourceRefGeneratorRef:
    r"""
    GeneratorRef points to a generator custom resource.

    Deprecated: The generatorRef is not implemented in .data[].
    this will be removed with v1.

    Attributes
    ----------
    apiVersion : str, default is "generators.external-secrets.io/v1alpha1", optional
        Specify the apiVersion of the generator resource
    kind : str, default is Undefined, required
        Specify the Kind of the generator resource
    name : str, default is Undefined, required
        Specify the name of the generator resource
    """
    apiVersion?: str = "generators.external-secrets.io/v1alpha1"
    kind: "ACRAccessToken" | "ClusterGenerator" | "ECRAuthorizationToken" | "Fake" | "GCRAccessToken" | "GithubAccessToken" | "QuayAccessToken" | "Password" | "STSSessionToken" | "UUID" | "VaultDynamicSecret" | "Webhook" | "Grafana"

    name: str

    check:
        len(name) <= 253
        len(name) >= 1
        _regex_match(str(name), r"^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$")

schema ExternalSecretsIoV1beta1ExternalSecretSpecDataItems0SourceRefStoreRef:
    r"""
    SecretStoreRef defines which SecretStore to fetch the ExternalSecret data.

    Attributes
    ----------
    kind : str, default is Undefined, optional
        Kind of the SecretStore resource (SecretStore or ClusterSecretStore)
        Defaults to `SecretStore`
    name : str, default is Undefined, optional
        Name of the SecretStore resource
    """
    kind?: "SecretStore" | "ClusterSecretStore"
    name?: str

    check:
        len(name) <= 253 if name
        len(name) >= 1 if name
        _regex_match(str(name), r"^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$") if name

schema ExternalSecretsIoV1beta1ExternalSecretSpecSecretStoreRef:
    r"""
    SecretStoreRef defines which SecretStore to fetch the ExternalSecret data.

    Attributes
    ----------
    kind : str, default is Undefined, optional
        Kind of the SecretStore resource (SecretStore or ClusterSecretStore)
        Defaults to `SecretStore`
    name : str, default is Undefined, optional
        Name of the SecretStore resource
    """
    kind?: "SecretStore" | "ClusterSecretStore"
    name?: str

    check:
        len(name) <= 253 if name
        len(name) >= 1 if name
        _regex_match(str(name), r"^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$") if name

schema ExternalSecretsIoV1beta1ExternalSecretSpecTarget:
    r"""
    ExternalSecretTarget defines the Kubernetes Secret to be created
    There can be only one target per ExternalSecret.

    Attributes
    ----------
    creationPolicy : str, default is "Owner", optional
        CreationPolicy defines rules on how to create the resulting Secret.
        Defaults to "Owner"
    deletionPolicy : str, default is "Retain", optional
        DeletionPolicy defines rules on how to delete the resulting Secret.
        Defaults to "Retain"
    immutable : bool, default is Undefined, optional
        Immutable defines if the final secret will be immutable
    name : str, default is Undefined, optional
        The name of the Secret resource to be managed.
        Defaults to the .metadata.name of the ExternalSecret resource
    template : ExternalSecretsIoV1beta1ExternalSecretSpecTargetTemplate, default is Undefined, optional
        template
    """
    creationPolicy?: "Owner" | "Orphan" | "Merge" | "None" = "Owner"
    deletionPolicy?: "Delete" | "Merge" | "Retain" = "Retain"
    immutable?: bool
    name?: str
    template?: ExternalSecretsIoV1beta1ExternalSecretSpecTargetTemplate

    check:
        len(name) <= 253 if name
        len(name) >= 1 if name
        _regex_match(str(name), r"^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$") if name

schema ExternalSecretsIoV1beta1ExternalSecretSpecTargetTemplate:
    r"""
    Template defines a blueprint for the created Secret resource.

    Attributes
    ----------
    data : {str:str}, default is Undefined, optional
        data
    engineVersion : str, default is "v2", optional
        EngineVersion specifies the template engine version
        that should be used to compile/execute the
        template specified in .data and .templateFrom[].
    mergePolicy : str, default is "Replace", optional
        merge policy
    metadata : ExternalSecretsIoV1beta1ExternalSecretSpecTargetTemplateMetadata, default is Undefined, optional
        metadata
    templateFrom : [ExternalSecretsIoV1beta1ExternalSecretSpecTargetTemplateTemplateFromItems0], default is Undefined, optional
        template from
    $type : str, default is Undefined, optional
        type
    """
    data?: {str:str}
    engineVersion?: "v2" = "v2"
    mergePolicy?: "Replace" | "Merge" = "Replace"
    metadata?: ExternalSecretsIoV1beta1ExternalSecretSpecTargetTemplateMetadata
    templateFrom?: [ExternalSecretsIoV1beta1ExternalSecretSpecTargetTemplateTemplateFromItems0]
    $type?: str

schema ExternalSecretsIoV1beta1ExternalSecretSpecTargetTemplateMetadata:
    r"""
    ExternalSecretTemplateMetadata defines metadata fields for the Secret blueprint.

    Attributes
    ----------
    annotations : {str:str}, default is Undefined, optional
        annotations
    labels : {str:str}, default is Undefined, optional
        labels
    """
    annotations?: {str:str}
    labels?: {str:str}

schema ExternalSecretsIoV1beta1ExternalSecretSpecTargetTemplateTemplateFromItems0:
    r"""
    external secrets io v1beta1 external secret spec target template template from items0

    Attributes
    ----------
    configMap : ExternalSecretsIoV1beta1ExternalSecretSpecTargetTemplateTemplateFromItems0ConfigMap, default is Undefined, optional
        config map
    literal : str, default is Undefined, optional
        literal
    secret : ExternalSecretsIoV1beta1ExternalSecretSpecTargetTemplateTemplateFromItems0Secret, default is Undefined, optional
        secret
    target : str, default is "Data", optional
        target
    """
    configMap?: ExternalSecretsIoV1beta1ExternalSecretSpecTargetTemplateTemplateFromItems0ConfigMap
    literal?: str
    secret?: ExternalSecretsIoV1beta1ExternalSecretSpecTargetTemplateTemplateFromItems0Secret
    target?: "Data" | "Annotations" | "Labels" = "Data"

schema ExternalSecretsIoV1beta1ExternalSecretSpecTargetTemplateTemplateFromItems0ConfigMap:
    r"""
    external secrets io v1beta1 external secret spec target template template from items0 config map

    Attributes
    ----------
    items : [ExternalSecretsIoV1beta1ExternalSecretSpecTargetTemplateTemplateFromItems0ConfigMapItemsItems0], default is Undefined, required
        A list of keys in the ConfigMap/Secret to use as templates for Secret data
    name : str, default is Undefined, required
        The name of the ConfigMap/Secret resource
    """
    items: [ExternalSecretsIoV1beta1ExternalSecretSpecTargetTemplateTemplateFromItems0ConfigMapItemsItems0]

    name: str

    check:
        len(name) <= 253
        len(name) >= 1
        _regex_match(str(name), r"^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$")

schema ExternalSecretsIoV1beta1ExternalSecretSpecTargetTemplateTemplateFromItems0ConfigMapItemsItems0:
    r"""
    external secrets io v1beta1 external secret spec target template template from items0 config map items items0

    Attributes
    ----------
    key : str, default is Undefined, required
        A key in the ConfigMap/Secret
    templateAs : str, default is "Values", optional
        template as
    """
    key: str

    templateAs?: "Values" | "KeysAndValues" = "Values"

    check:
        len(key) <= 253
        len(key) >= 1
        _regex_match(str(key), r"^[-._a-zA-Z0-9]+$")

schema ExternalSecretsIoV1beta1ExternalSecretSpecTargetTemplateTemplateFromItems0Secret:
    r"""
    external secrets io v1beta1 external secret spec target template template from items0 secret

    Attributes
    ----------
    items : [ExternalSecretsIoV1beta1ExternalSecretSpecTargetTemplateTemplateFromItems0SecretItemsItems0], default is Undefined, required
        A list of keys in the ConfigMap/Secret to use as templates for Secret data
    name : str, default is Undefined, required
        The name of the ConfigMap/Secret resource
    """
    items: [ExternalSecretsIoV1beta1ExternalSecretSpecTargetTemplateTemplateFromItems0SecretItemsItems0]

    name: str

    check:
        len(name) <= 253
        len(name) >= 1
        _regex_match(str(name), r"^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$")

schema ExternalSecretsIoV1beta1ExternalSecretSpecTargetTemplateTemplateFromItems0SecretItemsItems0:
    r"""
    external secrets io v1beta1 external secret spec target template template from items0 secret items items0

    Attributes
    ----------
    key : str, default is Undefined, required
        A key in the ConfigMap/Secret
    templateAs : str, default is "Values", optional
        template as
    """
    key: str

    templateAs?: "Values" | "KeysAndValues" = "Values"

    check:
        len(key) <= 253
        len(key) >= 1
        _regex_match(str(key), r"^[-._a-zA-Z0-9]+$")

schema ExternalSecretsIoV1beta1ExternalSecretStatus:
    r"""
    external secrets io v1beta1 external secret status

    Attributes
    ----------
    binding : ExternalSecretsIoV1beta1ExternalSecretStatusBinding, default is Undefined, optional
        binding
    conditions : [ExternalSecretsIoV1beta1ExternalSecretStatusConditionsItems0], default is Undefined, optional
        conditions
    refreshTime : str, default is Undefined, optional
        refreshTime is the time and date the external secret was fetched and
        the target secret updated
    syncedResourceVersion : str, default is Undefined, optional
        SyncedResourceVersion keeps track of the last synced version
    """
    binding?: ExternalSecretsIoV1beta1ExternalSecretStatusBinding
    conditions?: [ExternalSecretsIoV1beta1ExternalSecretStatusConditionsItems0]
    refreshTime?: str
    syncedResourceVersion?: str

schema ExternalSecretsIoV1beta1ExternalSecretStatusBinding:
    r"""
    Binding represents a servicebinding.io Provisioned Service reference to the secret

    Attributes
    ----------
    name : str, default is Undefined, optional
        Name of the referent.
        This field is effectively required, but due to backwards compatibility is
        allowed to be empty. Instances of this type with an empty value here are
        almost certainly wrong.
        More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
    """
    name?: str = ""

schema ExternalSecretsIoV1beta1ExternalSecretStatusConditionsItems0:
    r"""
    external secrets io v1beta1 external secret status conditions items0

    Attributes
    ----------
    lastTransitionTime : str, default is Undefined, optional
        last transition time
    message : str, default is Undefined, optional
        message
    reason : str, default is Undefined, optional
        reason
    status : str, default is Undefined, required
        status
    $type : str, default is Undefined, required
        type
    """
    lastTransitionTime?: str
    message?: str
    reason?: str
    status: str

    $type: str

