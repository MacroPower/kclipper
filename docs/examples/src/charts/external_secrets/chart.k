"""
This file was generated by the KCL auto-gen tool. DO NOT EDIT.
Editing this file might prove futile when you re-run the KCL auto-gen generate command.
"""

import helm

schema Chart(helm.Chart):
    r"""
    All possible chart configuration, inheriting from `helm.Chart(helm.ChartBase)`.

    Attributes
    ----------
    values : Values | any, optional, default is {crds = {createClusterExternalSecret = False, createClusterGenerator = False, createClusterSecretStore = False, createPushSecret = True}, installCRDs = True}
    chart : str, required, default is "external-secrets"
    repoURL : str, required, default is "https://charts.external-secrets.io/"
    targetRevision : str, optional, default is "0.18.2"
    """
    values?: Values | any = {crds = {createClusterExternalSecret = False, createClusterGenerator = False, createClusterSecretStore = False, createPushSecret = True}, installCRDs = True}
    chart: str = "external-secrets"
    repoURL: str = "https://charts.external-secrets.io/"
    targetRevision?: str = "0.18.2"

