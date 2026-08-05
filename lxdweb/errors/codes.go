package errors

const (
	ERR_SUCCESS = 0

	ERR_NODE_NOT_FOUND       = 1001
	ERR_NODE_ALREADY_EXISTS  = 1002
	ERR_NODE_CONNECTION_FAIL = 1003
	ERR_NODE_INVALID_CONFIG  = 1004

	ERR_CONTAINER_NOT_FOUND      = 2001
	ERR_CONTAINER_ALREADY_EXISTS = 2002
	ERR_CONTAINER_RUNNING        = 2003
	ERR_CONTAINER_STOPPED        = 2004
	ERR_CONTAINER_NOT_RUNNING    = 2005
	ERR_CONTAINER_STATUS_UNKNOWN = 2006
	ERR_CONTAINER_CONFIG_INVALID = 2007

	ERR_DB_QUERY_FAIL       = 4001
	ERR_DB_INSERT_FAIL      = 4002
	ERR_DB_UPDATE_FAIL      = 4003
	ERR_DB_DELETE_FAIL      = 4004
	ERR_DB_TRANSACTION_FAIL = 4005
	ERR_DB_RECORD_NOT_FOUND = 4006
	ERR_DB_DUPLICATE_ENTRY  = 4007

	ERR_AUTH_FAILED         = 5001
	ERR_AUTH_INVALID_TOKEN  = 5002
	ERR_AUTH_EXPIRED        = 5003
	ERR_AUTH_NO_PERMISSION  = 5004
	ERR_AUTH_INVALID_CAPTCHA = 5005

	ERR_SYNC_FAILED       = 6001
	ERR_SYNC_IN_PROGRESS  = 6002
	ERR_SYNC_TIMEOUT      = 6003

	ERR_CONFIG_LOAD_FAIL  = 7001
	ERR_CONFIG_SAVE_FAIL  = 7002
	ERR_CONFIG_INVALID    = 7003

	ERR_SYSTEM_COMMAND_FAIL = 8001
	ERR_SYSTEM_NETWORK_FAIL = 8002
	ERR_SYSTEM_DISK_FULL    = 8003
	ERR_SYSTEM_PERMISSION   = 8004
	ERR_SYSTEM_TIMEOUT      = 8005
	ERR_SYSTEM_UNKNOWN      = 8006
)

var ErrorMessages = map[int]string{
	ERR_SUCCESS: "Success",

	ERR_NODE_NOT_FOUND:       "Node not found",
	ERR_NODE_ALREADY_EXISTS:  "Node already exists",
	ERR_NODE_CONNECTION_FAIL: "Node connection failed",
	ERR_NODE_INVALID_CONFIG:  "Node configuration invalid",

	ERR_CONTAINER_NOT_FOUND:      "Container not found",
	ERR_CONTAINER_ALREADY_EXISTS: "Container already exists",
	ERR_CONTAINER_RUNNING:        "Container is running",
	ERR_CONTAINER_STOPPED:        "Container is stopped",
	ERR_CONTAINER_NOT_RUNNING:    "Container is not running",
	ERR_CONTAINER_STATUS_UNKNOWN: "Container status unknown",
	ERR_CONTAINER_CONFIG_INVALID: "Container configuration invalid",

	ERR_DB_QUERY_FAIL:       "Database query failed",
	ERR_DB_INSERT_FAIL:      "Database insert failed",
	ERR_DB_UPDATE_FAIL:      "Database update failed",
	ERR_DB_DELETE_FAIL:      "Database delete failed",
	ERR_DB_TRANSACTION_FAIL: "Database transaction failed",
	ERR_DB_RECORD_NOT_FOUND: "Database record not found",
	ERR_DB_DUPLICATE_ENTRY:  "Database duplicate entry",

	ERR_AUTH_FAILED:          "Authentication failed",
	ERR_AUTH_INVALID_TOKEN:   "Invalid token",
	ERR_AUTH_EXPIRED:         "Token expired",
	ERR_AUTH_NO_PERMISSION:   "No permission",
	ERR_AUTH_INVALID_CAPTCHA: "Invalid captcha",

	ERR_SYNC_FAILED:      "Sync failed",
	ERR_SYNC_IN_PROGRESS: "Sync already in progress",
	ERR_SYNC_TIMEOUT:     "Sync timeout",

	ERR_CONFIG_LOAD_FAIL: "Config load failed",
	ERR_CONFIG_SAVE_FAIL: "Config save failed",
	ERR_CONFIG_INVALID:   "Config invalid",

	ERR_SYSTEM_COMMAND_FAIL: "System command failed",
	ERR_SYSTEM_NETWORK_FAIL: "System network failed",
	ERR_SYSTEM_DISK_FULL:    "System disk full",
	ERR_SYSTEM_PERMISSION:   "System permission denied",
	ERR_SYSTEM_TIMEOUT:      "System timeout",
	ERR_SYSTEM_UNKNOWN:      "Unknown system error",
}

func GetErrorMessage(code int) string {
	if msg, ok := ErrorMessages[code]; ok {
		return msg
	}
	return "Unknown error"
}

