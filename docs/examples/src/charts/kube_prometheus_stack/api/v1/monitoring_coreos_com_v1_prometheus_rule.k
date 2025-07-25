"""
This file was generated by the KCL auto-gen tool. DO NOT EDIT.
Editing this file might prove futile when you re-run the KCL auto-gen generate command.
"""

import regex
import k8s.apimachinery.pkg.apis.meta.v1

_regex_match = regex.match

schema PrometheusRule:
    r"""
    The `PrometheusRule` custom resource definition (CRD) defines [alerting](https://prometheus.io/docs/prometheus/latest/configuration/alerting_rules/) and [recording](https://prometheus.io/docs/prometheus/latest/configuration/recording_rules/) rules to be evaluated by `Prometheus` or `ThanosRuler` objects.

    `Prometheus` and `ThanosRuler` objects select `PrometheusRule` objects using label and namespace selectors.

    Attributes
    ----------
    apiVersion : str, default is "monitoring.coreos.com/v1", required
        APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources
    kind : str, default is "PrometheusRule", required
        Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds
    metadata : v1.ObjectMeta, default is Undefined, optional
        metadata
    spec : MonitoringCoreosComV1PrometheusRuleSpec, default is Undefined, required
        spec
    """
    apiVersion: "monitoring.coreos.com/v1" = "monitoring.coreos.com/v1"

    kind: "PrometheusRule" = "PrometheusRule"

    metadata?: v1.ObjectMeta
    spec: MonitoringCoreosComV1PrometheusRuleSpec

schema MonitoringCoreosComV1PrometheusRuleSpec:
    r"""
    Specification of desired alerting rule definitions for Prometheus.

    Attributes
    ----------
    groups : [MonitoringCoreosComV1PrometheusRuleSpecGroupsItems0], default is Undefined, optional
        Content of Prometheus rule file
    """
    groups?: [MonitoringCoreosComV1PrometheusRuleSpecGroupsItems0]

schema MonitoringCoreosComV1PrometheusRuleSpecGroupsItems0:
    r"""
    RuleGroup is a list of sequentially evaluated recording and alerting rules.

    Attributes
    ----------
    interval : str, default is Undefined, optional
        Interval determines how often rules in the group are evaluated.
    labels : {str:str}, default is Undefined, optional
        Labels to add or overwrite before storing the result for its rules.
        The labels defined at the rule level take precedence.

        It requires Prometheus >= 3.0.0.
        The field is ignored for Thanos Ruler.
    limit : int, default is Undefined, optional
        Limit the number of alerts an alerting rule and series a recording
        rule can produce.
        Limit is supported starting with Prometheus >= 2.31 and Thanos Ruler >= 0.24.
    name : str, default is Undefined, required
        Name of the rule group.
    partial_response_strategy : str, default is Undefined, optional
        PartialResponseStrategy is only used by ThanosRuler and will
        be ignored by Prometheus instances.
        More info: https://github.com/thanos-io/thanos/blob/main/docs/components/rule.md#partial-response
    query_offset : str, default is Undefined, optional
        Defines the offset the rule evaluation timestamp of this particular group by the specified duration into the past.

        It requires Prometheus >= v2.53.0.
        It is not supported for ThanosRuler.
    rules : [MonitoringCoreosComV1PrometheusRuleSpecGroupsItems0RulesItems0], default is Undefined, optional
        List of alerting and recording rules.
    """
    interval?: str
    labels?: {str:str}
    limit?: int
    name: str

    partial_response_strategy?: str
    query_offset?: str
    rules?: [MonitoringCoreosComV1PrometheusRuleSpecGroupsItems0RulesItems0]

    check:
        _regex_match(str(interval), r"^(0|(([0-9]+)y)?(([0-9]+)w)?(([0-9]+)d)?(([0-9]+)h)?(([0-9]+)m)?(([0-9]+)s)?(([0-9]+)ms)?)$") if interval
        len(name) >= 1
        _regex_match(str(partial_response_strategy), r"^(?i)(abort|warn)?$") if partial_response_strategy
        _regex_match(str(query_offset), r"^(0|(([0-9]+)y)?(([0-9]+)w)?(([0-9]+)d)?(([0-9]+)h)?(([0-9]+)m)?(([0-9]+)s)?(([0-9]+)ms)?)$") if query_offset

schema MonitoringCoreosComV1PrometheusRuleSpecGroupsItems0RulesItems0:
    r"""
    Rule describes an alerting or recording rule
    See Prometheus documentation: [alerting](https://www.prometheus.io/docs/prometheus/latest/configuration/alerting_rules/) or [recording](https://www.prometheus.io/docs/prometheus/latest/configuration/recording_rules/#recording-rules) rule

    Attributes
    ----------
    alert : str, default is Undefined, optional
        Name of the alert. Must be a valid label value.
        Only one of `record` and `alert` must be set.
    annotations : {str:str}, default is Undefined, optional
        Annotations to add to each alert.
        Only valid for alerting rules.
    expr : int | str, default is Undefined, required
        PromQL expression to evaluate.
    $for : str, default is Undefined, optional
        Alerts are considered firing once they have been returned for this long.
    keep_firing_for : str, default is Undefined, optional
        KeepFiringFor defines how long an alert will continue firing after the condition that triggered it has cleared.
    labels : {str:str}, default is Undefined, optional
        Labels to add or overwrite.
    record : str, default is Undefined, optional
        Name of the time series to output to. Must be a valid metric name.
        Only one of `record` and `alert` must be set.
    """
    alert?: str
    annotations?: {str:str}
    expr: int | str

    $for?: str
    keep_firing_for?: str
    labels?: {str:str}
    record?: str

    check:
        _regex_match(str($for), r"^(0|(([0-9]+)y)?(([0-9]+)w)?(([0-9]+)d)?(([0-9]+)h)?(([0-9]+)m)?(([0-9]+)s)?(([0-9]+)ms)?)$") if $for
        len(keep_firing_for) >= 1 if keep_firing_for
        _regex_match(str(keep_firing_for), r"^(0|(([0-9]+)y)?(([0-9]+)w)?(([0-9]+)d)?(([0-9]+)h)?(([0-9]+)m)?(([0-9]+)s)?(([0-9]+)ms)?)$") if keep_firing_for

