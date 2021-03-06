#!/usr/bin/env bash -x
#
# Copyright 2018-2020, CS Systemes d'Information, http://csgroup.eu
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# Deploys a DCOS bootstrap/upgrade server with minimum requirements
#
# This script has to be executed on the bootstrap/upgrade server


# Redirects outputs to dcos_prepare_bootstrap.log
rm -f /opt/safescale/var/log/dcos_prepare_bootstrap.log
exec 1<&-
exec 2<&-
exec 1<>/opt/safescale/var/log/dcos_prepare_bootstrap.log
exec 2>&1

{{ .reserved_BashLibrary }}

# Download if needed the file dcos_generate_config.sh and executes it
download_dcos_config_generator() {
    [ ! -f ${SF_VARDIR}/dcos/dcos_generate_config.sh ] && {
        echo "-------------------------------"
        echo "download_dcos_config_generator:"
        echo "-------------------------------"
        local URL=https://downloads.dcos.io/dcos/stable/dcos_generate_config.sh
        sfRetry 14m 5 "curl -qkSsL -o ${SF_VARDIR}/dcos/dcos_generate_config.sh $URL" || exit 200
    }
    echo "dcos_generate_config.sh successfully downloaded."
    exit 0
}
export -f download_dcos_config_generator

download_dcos_bin() {
    [ ! -f ${SF_VARDIR}/dcos/genconf/serve/dcos.bin ] && {
        echo "------------------"
        echo "download_dcos_bin:"
        echo "------------------"
        local URL=https://downloads.dcos.io/binaries/cli/linux/x86-64/dcos-1.13/dcos
        sfRetry 5m 5 "curl -qkSsL -o ${SF_VARDIR}/dcos/genconf/serve/dcos.bin $URL" || exit 201
    }
    echo "dcos cli successfully downloaded."
    exit 0
}
export -f download_dcos_bin

download_kubectl_bin() {
    [ ! -f ${SF_VARDIR}/dcos/genconf/serve/kubectl.bin ] && {
        echo "---------------------"
        echo "download_kubectl_bin:"
        echo "---------------------"
        local VERSION=(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)
        local URL=https://storage.googleapis.com/kubernetes-release/release/${VERSION}/bin/linux/amd64/kubectl
        sfRetry 2m 5 "curl -qkSsL -o ${SF_VARDIR}/dcos/genconf/serve/kubectl.bin $URL" || exit 202
    }
    echo "kubectl successfully downloaded."
    exit 0
}
export -f download_kubectl_bin

download_nginx_image() {
    echo "---------------------"
    echo "download_nginx_image:"
    echo "---------------------"
    sfRetry 1m 5 "systemctl status docker || systemctl restart docker" && \
    sfRetry 8m 5 "docker pull nginx:latest" || exit 203
}
export -f download_nginx_image

sfRetry 3m 5 "yum makecache" || exit 204

mkdir -p ${SF_VARDIR}/dcos/genconf/serve/docker && \
cd ${SF_VARDIR}/dcos && \
sfRetry 3m 5 "sfYum install -y wget curl time jq unzip" || exit 204

# Launch downloads in parallel
sfAsyncStart DDCG 15m bash -c download_dcos_config_generator
sfAsyncStart DDB 10m bash -c download_dcos_bin
sfAsyncStart DKB 10m bash -c download_kubectl_bin
sfAsyncStart DNI 10m bash -c download_nginx_image

# Install requirements for DCOS environment
{{ .reserved_CommonRequirements }}

# Awaits download of DCOS configuration generator
echo "Waiting for download_dcos_config_generator..."
sfAsyncWait DDCG || exit 205

cat >${SF_VARDIR}/dcos/genconf/ip-detect <<-'EOF'
#!/bin/sh
#
# Detects the IP address on the LAN for each DCOS host, using the first master IP
IP=$(ip route show to match {{ index .ClusterMasterIPs 0 }} | grep -Eo '[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}' | tail -1)
[ ! -z "$IP" ] && echo $IP && exit 0

exit 1
EOF
chmod a+rx-w ${SF_VARDIR}/dcos/genconf/ip-detect

cat >${SF_VARDIR}/dcos/genconf/private_ssh_key <<'EOF'
{{ .SSHPrivateKey }}
EOF
cat >${SF_VARDIR}/dcos/genconf/public_ssh_key <<'EOF'
{{ .SSHPublicKey }}
EOF
chmod 0600 ${SF_VARDIR}/dcos/genconf/*_ssh_key

CLUSTER_ADMIN_UID=$(id -u cladm)
PEM_ENCODED_PUBLIC_KEY="$(openssl rsa -in ${SF_VARDIR}/dcos/genconf/private_ssh_key -pubout 2>/dev/null | awk 'NF {sub(/\r/, ""); printf "%s\\n",$0;}')"

cat >${SF_VARDIR}/dcos/genconf/config.yaml <<-EOF
bootstrap_url: http://{{.HostIP}}:10080
cluster_name: {{ .ClusterName }}
exhibitor_storage_backend: static
master_discovery: static
master_list:
{{ range .ClusterMasterIPs }}
- {{.}}
{{ end }}
{{ if .DNSServers }}
resolvers:
  {{ range .DNSServers }}
  - {{.}}
  {{ end }}
{{ end }}
ssh_key_path: genconf/private_ssh_key
ssh_user: cladm
ssh_port: 22
superuser_service_account_uid: ${CLUSTER_ADMIN_UID}
superuser_service_account_public_key: "${PEM_ENCODED_PUBLIC_KEY}"
oauth_enabled: 'false'
use_proxy: 'false'
enable_ipv6: 'false'
dcos_l4lb_enable_ipv6: 'false'
EOF


( cd ${SF_VARDIR}/dcos ; bash ${SF_VARDIR}/dcos/dcos_generate_config.sh ) || exit 206

# Awaits pull of docker nginx image
echo "Waiting for docker nginx image..."
sfAsyncWait DNI || exit 207

# Awaits the download of DCOS binary
echo "Waiting for download_dcos_binary..."
sfAsyncWait DDB || exit 208

# Awaits the download of kubectl binary
echo "Waiting for download_kubectl_binary..."
sfAsyncWait DKB || exit 209

# Starts nginx server
mkdir -p ${SF_ETCDIR}/dcos/
cat >${SF_ETCDIR}/dcos/nginx.docker-compose.yml <<-EOF
version: "2.1"

services:
    bootstrap:
        image: nginx
        volumes:
            - ${SF_VARDIR}/dcos/genconf/serve:/usr/share/nginx/html:ro
        ports:
            - {{ .HostIP }}:10080:80
        restart: always
EOF
docker-compose -f ${SF_ETCDIR}/dcos/nginx.docker-compose.yml -p dcos4safescale up -d

echo
echo "Bootstrap prepared successfully."
exit 0
