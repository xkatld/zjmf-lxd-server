package errors

const (
	ERR_SUCCESS = 0

	ERR_CONTAINER_NOT_FOUND      = 1001
	ERR_CONTAINER_ALREADY_EXISTS = 1002
	ERR_CONTAINER_RUNNING        = 1003
	ERR_CONTAINER_STOPPED        = 1004
	ERR_CONTAINER_NOT_RUNNING    = 1005
	ERR_CONTAINER_STATUS_UNKNOWN = 1006
	ERR_CONTAINER_CONFIG_INVALID = 1007
	ERR_CONTAINER_NOT_PAUSED     = 1008

	ERR_LXC_CREATE_FAIL    = 2001
	ERR_LXC_START_FAIL     = 2002
	ERR_LXC_STOP_FAIL      = 2003
	ERR_LXC_DELETE_FAIL    = 2004
	ERR_LXC_RESTART_FAIL   = 2005
	ERR_LXC_EXEC_FAIL      = 2006
	ERR_LXC_QUERY_FAIL     = 2007
	ERR_LXC_CONFIG_FAIL    = 2008
	ERR_LXC_SNAPSHOT_FAIL  = 2009
	ERR_LXC_RESTORE_FAIL   = 2010
	ERR_LXC_LIST_FAIL      = 2011
	ERR_LXC_PUBLISH_FAIL   = 2012
	ERR_LXC_LAUNCH_FAIL    = 2013
	ERR_LXC_COPY_FAIL      = 2014
	ERR_LXC_FILE_PUSH_FAIL = 2015
	ERR_LXC_PAUSE_FAIL     = 2016
	ERR_LXC_RESUME_FAIL    = 2017

	ERR_NAT_RULE_NOT_FOUND    = 3001
	ERR_NAT_RULE_ADD_FAIL     = 3002
	ERR_NAT_RULE_DELETE_FAIL  = 3003
	ERR_NAT_IPTABLES_FAIL     = 3004
	ERR_NAT_PORT_IN_USE       = 3005
	ERR_NAT_QUERY_FAIL        = 3006
	ERR_NAT_CLEANUP_FAIL      = 3007
	ERR_NAT_EXTERNAL_IP_EMPTY = 3008
	ERR_SSH_QUERY_FAIL        = 3009
	ERR_SSH_CLEANUP_FAIL      = 3010
	ERR_SSH_RULE_ADD_FAIL     = 3011

	ERR_IPV6_BINDING_NOT_FOUND = 4001
	ERR_IPV6_BINDING_EXISTS    = 4002
	ERR_IPV6_ROUTE_ADD_FAIL    = 4003
	ERR_IPV6_ROUTE_DELETE_FAIL = 4004
	ERR_IPV6_POOL_EXHAUSTED    = 4005
	ERR_IPV6_QUERY_FAIL        = 4006
	ERR_IPV6_CLEANUP_FAIL      = 4007
	ERR_IPV6_DISABLED          = 4008

	ERR_DB_QUERY_FAIL      = 5001
	ERR_DB_INSERT_FAIL     = 5002
	ERR_DB_UPDATE_FAIL     = 5003
	ERR_DB_DELETE_FAIL     = 5004
	ERR_DB_TRANSACTION_FAIL = 5005
	ERR_DB_RECORD_NOT_FOUND = 5006
	ERR_DB_DUPLICATE_ENTRY  = 5007

	ERR_STORAGE_POOL_NOT_FOUND = 6001
	ERR_STORAGE_POOL_FULL      = 6002
	ERR_STORAGE_CREATE_FAIL    = 6003
	ERR_STORAGE_DELETE_FAIL    = 6004

	ERR_IMAGE_NOT_FOUND   = 7001
	ERR_IMAGE_PULL_FAIL   = 7002
	ERR_IMAGE_INVALID     = 7003
	ERR_IMAGE_ALIAS_FAIL  = 7004

	ERR_PASSWORD_RESET_FAIL    = 8001
	ERR_PASSWORD_SCRIPT_FAIL   = 8002
	ERR_PASSWORD_INVALID       = 8003

	ERR_TASK_CREATE_FAIL   = 9001
	ERR_TASK_NOT_FOUND     = 9002
	ERR_TASK_ALREADY_RUN   = 9003
	ERR_TASK_TIMEOUT       = 9004
	ERR_TASK_CANCELLED     = 9005
	ERR_TASK_RETRY_EXCEED  = 9006

	ERR_SYSTEM_COMMAND_FAIL = 10001
	ERR_SYSTEM_NETWORK_FAIL = 10002
	ERR_SYSTEM_DISK_FULL    = 10003
	ERR_SYSTEM_PERMISSION   = 10004
	ERR_SYSTEM_TIMEOUT      = 10005
	ERR_SYSTEM_UNKNOWN      = 10006

	ERR_PROXY_RULE_NOT_FOUND    = 11001
	ERR_PROXY_RULE_EXISTS       = 11002
	ERR_PROXY_DOMAIN_INVALID    = 11003
	ERR_PROXY_CONTAINER_NO_IP   = 11004
	ERR_PROXY_CONFIG_GENERATE   = 11005
	ERR_PROXY_FILE_WRITE        = 11006
	ERR_PROXY_NGINX_TEST_FAIL   = 11007
	ERR_PROXY_NGINX_RELOAD_FAIL = 11008
	ERR_PROXY_DISABLED          = 11009
	ERR_PROXY_TEMPLATE_NOT_FOUND = 11010
	ERR_PROXY_TEMPLATE_PARSE    = 11011
)

var ErrorMessages = map[int]string{
	ERR_SUCCESS: "Success",

	ERR_CONTAINER_NOT_FOUND:      "Container not found",
	ERR_CONTAINER_ALREADY_EXISTS: "Container already exists",
	ERR_CONTAINER_RUNNING:        "Container is running",
	ERR_CONTAINER_STOPPED:        "Container is stopped",
	ERR_CONTAINER_NOT_RUNNING:    "Container is not running",
	ERR_CONTAINER_STATUS_UNKNOWN: "Container status unknown",
	ERR_CONTAINER_CONFIG_INVALID: "Container configuration invalid",

	ERR_LXC_CREATE_FAIL:    "LXC create failed",
	ERR_LXC_START_FAIL:     "LXC start failed",
	ERR_LXC_STOP_FAIL:      "LXC stop failed",
	ERR_LXC_DELETE_FAIL:    "LXC delete failed",
	ERR_LXC_RESTART_FAIL:   "LXC restart failed",
	ERR_LXC_EXEC_FAIL:      "LXC exec failed",
	ERR_LXC_QUERY_FAIL:     "LXC query failed",
	ERR_LXC_CONFIG_FAIL:    "LXC config failed",
	ERR_LXC_SNAPSHOT_FAIL:  "LXC snapshot failed",
	ERR_LXC_RESTORE_FAIL:   "LXC restore failed",
	ERR_LXC_LIST_FAIL:      "LXC list failed",
	ERR_LXC_PUBLISH_FAIL:   "LXC publish failed",
	ERR_LXC_LAUNCH_FAIL:    "LXC launch failed",
	ERR_LXC_COPY_FAIL:      "LXC copy failed",
	ERR_LXC_FILE_PUSH_FAIL: "LXC file push failed",

	ERR_NAT_RULE_NOT_FOUND:    "NAT rule not found",
	ERR_NAT_RULE_ADD_FAIL:     "NAT rule add failed",
	ERR_NAT_RULE_DELETE_FAIL:  "NAT rule delete failed",
	ERR_NAT_IPTABLES_FAIL:     "iptables command failed",
	ERR_NAT_PORT_IN_USE:       "NAT port already in use",
	ERR_NAT_QUERY_FAIL:        "NAT query failed",
	ERR_NAT_CLEANUP_FAIL:      "NAT cleanup failed",
	ERR_NAT_EXTERNAL_IP_EMPTY: "NAT external IP not configured",

	ERR_IPV6_BINDING_NOT_FOUND: "IPv6 binding not found",
	ERR_IPV6_BINDING_EXISTS:    "IPv6 binding already exists",
	ERR_IPV6_ROUTE_ADD_FAIL:    "IPv6 route add failed",
	ERR_IPV6_ROUTE_DELETE_FAIL: "IPv6 route delete failed",
	ERR_IPV6_POOL_EXHAUSTED:    "IPv6 address pool exhausted",
	ERR_IPV6_QUERY_FAIL:        "IPv6 query failed",
	ERR_IPV6_CLEANUP_FAIL:      "IPv6 cleanup failed",
	ERR_IPV6_DISABLED:          "IPv6 binding disabled",

	ERR_DB_QUERY_FAIL:       "Database query failed",
	ERR_DB_INSERT_FAIL:      "Database insert failed",
	ERR_DB_UPDATE_FAIL:      "Database update failed",
	ERR_DB_DELETE_FAIL:      "Database delete failed",
	ERR_DB_TRANSACTION_FAIL: "Database transaction failed",
	ERR_DB_RECORD_NOT_FOUND: "Database record not found",
	ERR_DB_DUPLICATE_ENTRY:  "Database duplicate entry",

	ERR_STORAGE_POOL_NOT_FOUND: "Storage pool not found",
	ERR_STORAGE_POOL_FULL:      "Storage pool full",
	ERR_STORAGE_CREATE_FAIL:    "Storage create failed",
	ERR_STORAGE_DELETE_FAIL:    "Storage delete failed",

	ERR_IMAGE_NOT_FOUND:  "Image not found",
	ERR_IMAGE_PULL_FAIL:  "Image pull failed",
	ERR_IMAGE_INVALID:    "Image invalid",
	ERR_IMAGE_ALIAS_FAIL: "Image alias failed",

	ERR_PASSWORD_RESET_FAIL:  "Password reset failed",
	ERR_PASSWORD_SCRIPT_FAIL: "Password script failed",
	ERR_PASSWORD_INVALID:     "Password invalid",

	ERR_TASK_CREATE_FAIL:  "Task create failed",
	ERR_TASK_NOT_FOUND:    "Task not found",
	ERR_TASK_ALREADY_RUN:  "Task already running",
	ERR_TASK_TIMEOUT:      "Task timeout",
	ERR_TASK_CANCELLED:    "Task cancelled",
	ERR_TASK_RETRY_EXCEED: "Task retry limit exceeded",

	ERR_SYSTEM_COMMAND_FAIL: "System command failed",
	ERR_SYSTEM_NETWORK_FAIL: "System network failed",
	ERR_SYSTEM_DISK_FULL:    "System disk full",
	ERR_SYSTEM_PERMISSION:   "System permission denied",
	ERR_SYSTEM_TIMEOUT:      "System timeout",
	ERR_SYSTEM_UNKNOWN:      "Unknown system error",

	ERR_PROXY_RULE_NOT_FOUND:    "Proxy rule not found",
	ERR_PROXY_RULE_EXISTS:       "Domain already exists",
	ERR_PROXY_DOMAIN_INVALID:    "Domain format invalid",
	ERR_PROXY_CONTAINER_NO_IP:   "Container has no IP address",
	ERR_PROXY_CONFIG_GENERATE:   "Config generation failed",
	ERR_PROXY_FILE_WRITE:        "Config file write failed",
	ERR_PROXY_NGINX_TEST_FAIL:   "Nginx config test failed",
	ERR_PROXY_NGINX_RELOAD_FAIL: "Nginx reload failed",
	ERR_PROXY_DISABLED:          "Proxy feature disabled",
	ERR_PROXY_TEMPLATE_NOT_FOUND: "Template file not found",
	ERR_PROXY_TEMPLATE_PARSE:    "Template parse failed",
}

func GetErrorMessage(code int) string {
	if msg, ok := ErrorMessages[code]; ok {
		return msg
	}
	return "Unknown error"
}

