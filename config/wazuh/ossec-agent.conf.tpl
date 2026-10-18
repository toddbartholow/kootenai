<!--
  Wazuh Agent Configuration Template for Kootenai VMs

  Variables to replace:
    {{.ManagerIP}}      - Wazuh Manager IP address
    {{.AgentName}}      - Agent name (typically VM hostname)
    {{.AgentGroup}}     - Agent group (typically pod-{podID})
    {{.MonitorPaths}}   - Paths to monitor via FIM
-->
<ossec_config>
  <client>
    <server>
      <address>{{.ManagerIP}}</address>
      <port>1514</port>
      <protocol>tcp</protocol>
    </server>
    <config-profile>{{.AgentGroup}}</config-profile>
    <notify_time>10</notify_time>
    <time-reconnect>60</time-reconnect>
    <auto_restart>yes</auto_restart>
  </client>

  <client_buffer>
    <disabled>no</disabled>
    <queue_size>5000</queue_size>
    <events_per_second>500</events_per_second>
  </client_buffer>

  <!-- File Integrity Monitoring -->
  <syscheck>
    <disabled>no</disabled>
    <frequency>300</frequency>
    <scan_on_start>yes</scan_on_start>
    <alert_new_files>yes</alert_new_files>
    <auto_ignore frequency="10" timeframe="3600">no</auto_ignore>

    <!-- Default monitored directories -->
    <directories realtime="yes" report_changes="yes" check_all="yes">/etc</directories>
    <directories realtime="yes" report_changes="yes" check_all="yes">/usr/bin</directories>
    <directories realtime="yes" report_changes="yes" check_all="yes">/usr/sbin</directories>
    <directories realtime="yes" report_changes="yes" check_all="yes">/bin</directories>
    <directories realtime="yes" report_changes="yes" check_all="yes">/sbin</directories>

    <!-- Lab-specific monitored paths -->
    {{range .MonitorPaths}}
    <directories realtime="yes" report_changes="yes" check_all="yes">{{.}}</directories>
    {{end}}

    <!-- Ignore frequently changing files -->
    <ignore>/etc/mtab</ignore>
    <ignore>/etc/resolv.conf</ignore>
    <ignore>/etc/adjtime</ignore>
    <ignore>/etc/passwd.lock</ignore>
    <ignore>/etc/shadow.lock</ignore>
    <ignore>/etc/gshadow.lock</ignore>
    <ignore>/etc/group.lock</ignore>
    <ignore type="sregex">.swp$</ignore>
    <ignore type="sregex">.swx$</ignore>
    <ignore type="sregex">~$</ignore>

    <!-- Scan system binaries weekly -->
    <directories check_all="yes">/boot</directories>

    <!-- Windows support (if applicable) -->
    <directories realtime="yes" report_changes="yes">%WINDIR%\System32\drivers\etc</directories>
    <directories realtime="yes" report_changes="yes">%WINDIR%\System32\config</directories>
  </syscheck>

  <!-- Rootcheck -->
  <rootcheck>
    <disabled>no</disabled>
    <check_files>yes</check_files>
    <check_trojans>yes</check_trojans>
    <check_dev>yes</check_dev>
    <check_sys>yes</check_sys>
    <check_pids>yes</check_pids>
    <check_ports>yes</check_ports>
    <check_if>yes</check_if>
    <frequency>43200</frequency>
    <rootkit_files>etc/shared/rootkit_files.txt</rootkit_files>
    <rootkit_trojans>etc/shared/rootkit_trojans.txt</rootkit_trojans>
    <skip_nfs>yes</skip_nfs>
  </rootcheck>

  <!-- Log Analysis -->
  <localfile>
    <log_format>syslog</log_format>
    <location>/var/log/syslog</location>
  </localfile>

  <localfile>
    <log_format>syslog</log_format>
    <location>/var/log/auth.log</location>
  </localfile>

  <localfile>
    <log_format>syslog</log_format>
    <location>/var/log/dpkg.log</location>
  </localfile>

  <localfile>
    <log_format>apache</log_format>
    <location>/var/log/apache2/access.log</location>
  </localfile>

  <localfile>
    <log_format>apache</log_format>
    <location>/var/log/apache2/error.log</location>
  </localfile>

  <localfile>
    <log_format>syslog</log_format>
    <location>/var/log/kern.log</location>
  </localfile>

  <!-- Command monitoring for lab objectives -->
  <localfile>
    <log_format>command</log_format>
    <command>df -P</command>
    <frequency>360</frequency>
  </localfile>

  <localfile>
    <log_format>full_command</log_format>
    <command>netstat -tulpn | sed 's/\([[:alnum:]]\+\)\ \+[[:digit:]]\+\ \+[[:digit:]]\+\ \+\(.*\):\([[:digit:]]*\)\ \+\([0-9\.\:\*]\+\).\telesp\ \+\([[:digit:]]*\/[[:alnum:]\-]*\).*/\1 \2 == \3 == \4 \5/' | sort -k 4 -g | sed 's/ == \(.*\) ==/:\1/' | sed 1,2d</command>
    <alias>netstat listening ports</alias>
    <frequency>360</frequency>
  </localfile>

  <localfile>
    <log_format>full_command</log_format>
    <command>last -n 20</command>
    <frequency>360</frequency>
  </localfile>

  <!-- Active Response -->
  <active-response>
    <disabled>no</disabled>
    <ca_store>etc/wpk_root.pem</ca_store>
  </active-response>

  <!-- Logging -->
  <logging>
    <log_format>plain</log_format>
  </logging>
</ossec_config>
