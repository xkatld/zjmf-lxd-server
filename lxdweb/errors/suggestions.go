package errors

var errorSuggestions = map[int]string{
	ERR_SUCCESS: "",

	ERR_NODE_NOT_FOUND:       "请检查节点 ID 是否正确",
	ERR_NODE_ALREADY_EXISTS:  "节点名称已存在，请使用其他名称",
	ERR_NODE_CONNECTION_FAIL: "请检查节点地址和 API Key 是否正确",
	ERR_NODE_INVALID_CONFIG:  "请检查节点配置参数是否完整",

	ERR_CONTAINER_NOT_FOUND:      "请检查容器名称是否正确",
	ERR_CONTAINER_ALREADY_EXISTS: "容器名称已被使用，请使用其他名称",
	ERR_CONTAINER_RUNNING:        "请先停止容器再执行此操作",
	ERR_CONTAINER_STOPPED:        "请先启动容器",
	ERR_CONTAINER_NOT_RUNNING:    "容器未运行，请检查容器状态",
	ERR_CONTAINER_STATUS_UNKNOWN: "请稍后重试，或联系管理员检查节点状态",
	ERR_CONTAINER_CONFIG_INVALID: "请检查容器配置参数是否正确",

	ERR_DB_QUERY_FAIL:       "请检查数据库连接和查询语句",
	ERR_DB_INSERT_FAIL:      "请检查数据是否重复或字段是否完整",
	ERR_DB_UPDATE_FAIL:      "请检查记录是否存在",
	ERR_DB_DELETE_FAIL:      "请检查记录是否存在或是否被其他数据引用",
	ERR_DB_TRANSACTION_FAIL: "请重试或联系管理员",
	ERR_DB_RECORD_NOT_FOUND: "请检查查询条件是否正确",
	ERR_DB_DUPLICATE_ENTRY:  "该记录已存在",

	ERR_AUTH_FAILED:          "用户名或密码错误",
	ERR_AUTH_INVALID_TOKEN:   "请重新登录",
	ERR_AUTH_EXPIRED:         "登录已过期，请重新登录",
	ERR_AUTH_NO_PERMISSION:   "您没有权限执行此操作",
	ERR_AUTH_INVALID_CAPTCHA: "验证码错误，请重新输入",

	ERR_SYNC_FAILED:      "同步失败，请检查节点连接",
	ERR_SYNC_IN_PROGRESS: "节点正在同步中，请稍后再试",
	ERR_SYNC_TIMEOUT:     "同步超时，请检查网络连接",

	ERR_CONFIG_LOAD_FAIL: "请检查配置文件是否存在和格式是否正确",
	ERR_CONFIG_SAVE_FAIL: "请检查文件权限和磁盘空间",
	ERR_CONFIG_INVALID:   "请检查配置项是否完整和正确",

	ERR_SYSTEM_COMMAND_FAIL: "请检查系统命令和权限",
	ERR_SYSTEM_NETWORK_FAIL: "请检查网络连接",
	ERR_SYSTEM_DISK_FULL:    "磁盘空间不足，请清理磁盘或扩容",
	ERR_SYSTEM_PERMISSION:   "权限不足，请使用管理员权限执行",
	ERR_SYSTEM_TIMEOUT:      "操作超时，请检查系统负载",
	ERR_SYSTEM_UNKNOWN:      "未知错误，请查看系统日志或联系管理员",
}

func GetSuggestion(errorCode int) string {
	if suggestion, ok := errorSuggestions[errorCode]; ok {
		return suggestion
	}
	return "请查看系统日志获取更多信息，或联系管理员"
}

