#!/bin/bash
#
# Copyright 2018-2020, CS Systemes d'Information, http://www.c-s.fr
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

#{{.Revision}}

{{.Header}}

print_error() {
    read line file <<<$(caller)
    echo "An error occurred in line $line of file $file:" "{"$(sed "${line}q;d" "$file")"}" >&2
    {{.ExitOnError}}
}
trap print_error ERR

fail() {
    echo "PROVISIONING_ERROR: $1"
    echo -n "$1,${LINUX_KIND},${FULL_VERSION_ID},$(hostname),$(date +%Y/%m/%d-%H:%M:%S)" >/opt/safescale/var/state/user_data.gwha.done
    exit $1
}

# Redirects outputs to /opt/safescale/log/user_data.phase2.log
exec 1<&-
exec 2<&-
exec 1<>/opt/safescale/var/log/user_data.phase2.log
exec 2>&1
set -x

# Includes the BashLibrary
{{ .BashLibrary }}

install_keepalived() {
    # Try installing network-scripts if available
    case $LINUX_KIND in
    redhat | rhel | centos | fedora)
        sfRetry 3m 5 "sfYum install -q -y network-scripts" || true
        ;;
    *) ;;

    esac

    case $LINUX_KIND in
    ubuntu | debian)
        sfRetry 3m 5 "sfApt update" || return 1
        sfRetry 3m 5 "sfApt -y install keepalived" || return 1
        ;;

    redhat | rhel | centos | fedora)
        sfRetry 3m 5 "sfYum install -q -y keepalived" || return 1
        ;;
    *)
        echo "Unsupported Linux distribution '$LINUX_KIND'!"
        return 1
        ;;
    esac

    NETMASK=$(echo {{ .CIDR }} | cut -d/ -f2)
    read IF_PR ignore <<<$(cat ${SF_VARDIR}/state/private_nics)
    read IF_PU ignore <<<$(cat ${SF_VARDIR}/state/public_nics)

    cat >/etc/keepalived/keepalived.conf <<-EOF
vrrp_instance vrrp_group_gws_internal {
    state {{ if eq .IsPrimaryGateway true }}MASTER{{ else }}BACKUP{{ end }}
    interface ${PR_IFs[0]}
    virtual_router_id 1
    priority {{ if eq .IsPrimaryGateway true }}151{{ else }}100{{ end }}
    nopreempt
    advert_int 2
    authentication {
        auth_type PASS
        auth_pass {{ .GatewayHAKeepalivedPassword }}
    }
{{ if eq .IsPrimaryGateway true }}
    # Unicast specific option, this is the IP of the interface keepalived listens on
    unicast_src_ip {{ .PrimaryGatewayPrivateIP }}
    # Unicast specific option, this is the IP of the peer instance
    unicast_peer {
        {{ .SecondaryGatewayPrivateIP }}
    }
{{ else }}
    unicast_src_ip {{ .SecondaryGatewayPrivateIP }}
    unicast_peer {
        {{ .PrimaryGatewayPrivateIP }}
    }
{{ end }}
    virtual_ipaddress {
        {{ .DefaultRouteIP }}/${NETMASK}
    }
}

# vrrp_instance vrrp_group_gws_external {
#     state BACKUP
#     interface ${PU_IF}
#     virtual_router_id 2
#     priority {{ if eq .IsPrimaryGateway true }}151{{ else }}100{{ end }}
#     nopreempt
#     advert_int 2
#     authentication {
#         auth_type PASS
#         auth_pass password
#     }
#     virtual_ipaddress {
#         {{ .EndpointIP }}/${NETMASK}
#     }
# }
EOF

    if [ "$(sfGetFact "use_systemd")" = "1" ]; then
        # Use systemd to ensure keepalived is restarted if network is restarted
        # (otherwise, keepalived is in undetermined state)
        mkdir -p /etc/systemd/system/keepalived.service.d
        if [[ $(sfGetFact "redhat_like") -eq 1 ]]; then
            cat >/etc/systemd/system/keepalived.service.d/override.conf <<EOF
[Unit]
Requires=network.service
PartOf=network.service
EOF
        else
            cat >/etc/systemd/system/keepalived.service.d/override.conf <<EOF
[Unit]
Requires=systemd-networkd.service
PartOf=systemd-networkd.service
EOF
        fi
        systemctl daemon-reload
    fi

    sfService enable keepalived || return 1

    op=-1
    msg=$(sfService restart keepalived 2>&1) && op=$? || true

    kop=-1
    echo $msg | grep "Unit network.service not found" && kop=$? || true

    if [[ op -ne 0 ]]; then
        if [[ kop -eq 0 ]]; then
            case $LINUX_KIND in
            redhat | rhel | centos | fedora)
                sfRetry 3m 5 "sfYum install -q -y network-scripts" || return 1
                ;;
            *) ;;

            esac
        fi
    fi

    sfService restart keepalived || return 1
    return 0
}


# ---- Main

{{- if .IsGateway }}
install_keepalived
{{ end }}

echo -n "0,linux,${LINUX_KIND},${FULL_VERSION_ID},$(hostname),$(date +%Y/%m/%d-%H:%M:%S)" >/opt/safescale/var/state/user_data.gwha.done

set +x
exit 0
