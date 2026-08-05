<?php

use app\common\logic\RunMap;
use app\common\model\HostModel;
use think\Db;

define('LXDSERVER_DEBUG', true);

function lxdserver_debug($message, $data = null) {
    if (!LXDSERVER_DEBUG) return;
    $log = '[LXD-DEBUG] ' . $message;
    if ($data !== null) {
        $log .= ' | Data: ' . json_encode($data, JSON_UNESCAPED_UNICODE);
    }
    error_log($log);
}

function lxdserver_MetaData()
{
    return [
        'DisplayName' => '魔方财务-LXD对接插件V1 by xkatld',
        'APIVersion'  => '1.1.0',
        'HelpDoc'     => 'https://github.com/xkatld/zjmf-lxd-server',
    ];
}

function lxdserver_ConfigOptions()
{
    return [
        'cpus' => [
            'type'        => 'text',
            'name'        => 'CPU核心数',
            'description' => 'CPU核心数量',
            'default'     => '1',
            'key'         => 'cpus',
        ],
        'memory' => [
            'type'        => 'text',
            'name'        => '内存',
            'description' => '内存大小[单位：MB GB]',
            'default'     => '256MB',
            'key'         => 'memory',
        ],
        'disk' => [
            'type'        => 'text',
            'name'        => '硬盘',
            'description' => '硬盘大小[单位：MB GB]',
            'default'     => '512MB',
            'key'         => 'disk',
        ],
        'image' => [
            'type'        => 'text',
            'name'        => '镜像',
            'description' => '系统镜像',
            'default'     => 'debian12',
            'key'         => 'image',
        ],
        'traffic_limit' => [
            'type'        => 'text',
            'name'        => '月流量限制',
            'description' => '单位：GB',
            'default'     => '100',
            'key'         => 'traffic_limit',
        ],
        'ingress' => [
            'type'        => 'text',
            'name'        => '入站带宽',
            'description' => '下载速度限制[单位：Mbit Gbit]',
            'default'     => '100Mbit',
            'key'         => 'ingress',
        ],
        'egress' => [
            'type'        => 'text',
            'name'        => '出站带宽',
            'description' => '上传速度限制[单位：Mbit Gbit]',
            'default'     => '100Mbit',
            'key'         => 'egress',
        ],
        'network_mode' => [
            'type'        => 'dropdown',
            'name'        => '网络模式',
            'description' => '选择容器网络运行模式',
            'default'     => 'mode1',
            'key'         => 'network_mode',
            'options'     => [
                'mode1' => '模式1：IPv4 NAT共享',
                'mode2' => '模式2：IPv4 NAT + IPv6独立',
            ],
        ],
        'nat_limit' => [
            'type'        => 'text',
            'name'        => 'NAT规则数量',
            'description' => '端口转发规则上限',
            'default'     => '5',
            'key'         => 'nat_limit',
        ],
        'udp_enabled' => [
            'type'        => 'dropdown',
            'name'        => 'UDP协议支持',
            'description' => '允许UDP端口转发',
            'default'     => 'false',
            'key'         => 'udp_enabled',
            'options'     => ['false' => '禁用', 'true' => '启用'],
        ],
        'cpu_allowance' => [
            'type'        => 'text',
            'name'        => 'CPU使用率限制',
            'description' => 'CPU占用百分比[0%-100%]',
            'default'     => '50%',
            'key'         => 'cpu_allowance',
        ],

        'memory_swap' => [
            'type'        => 'dropdown',
            'name'        => 'Swap开关',
            'description' => '虚拟内存开关',
            'default'     => 'true',
            'key'         => 'memory_swap',
            'options'     => ['true' => '启用', 'false' => '禁用'],
        ],

        'disk_io_limit' => [
            'type'        => 'text',
            'name'        => '磁盘IO限速',
            'description' => '读写速度限制[单位：MB]',
            'default'     => '100MB',
            'key'         => 'disk_io_limit',
        ],
        'max_processes' => [
            'type'        => 'text',
            'name'        => '最大进程数',
            'description' => '进程数量上限',
            'default'     => '512',
            'key'         => 'max_processes',
        ],
        'ipv6_limit' => [
            'type'        => 'text',
            'name'        => 'IPv6绑定数量',
            'description' => 'IPv6地址数量上限',
            'default'     => '1',
            'key'         => 'ipv6_limit',
        ],
        'ipv6_allow_delete' => [
            'type'        => 'dropdown',
            'name'        => 'IPv6允许删除',
            'description' => '是否可以删除IPv6地址',
            'default'     => 'true',
            'key'         => 'ipv6_allow_delete',
            'options'     => ['true' => '允许', 'false' => '禁止'],
        ],
        'proxy_enabled' => [
            'type'        => 'dropdown',
            'name'        => 'Nginx反向代理功能',
            'description' => '反向代理开关',
            'default'     => 'false',
            'key'         => 'proxy_enabled',
            'options'     => ['false' => '禁用', 'true' => '启用'],
        ],
        'proxy_limit' => [
            'type'        => 'text',
            'name'        => '反向代理域名数量',
            'description' => '域名绑定数量上限',
            'default'     => '1',
            'key'         => 'proxy_limit',
        ],
        'allow_nesting' => [
            'type'        => 'dropdown',
            'name'        => '嵌套虚拟化',
            'description' => '支持Docker等虚拟化',
            'default'     => 'true',
            'key'         => 'allow_nesting',
            'options'     => ['true' => '启用', 'false' => '禁用'],
        ],
        'privileged' => [
            'type'        => 'dropdown',
            'name'        => '特权模式',
            'description' => '特权容器开关',
            'default'     => 'false',
            'key'         => 'privileged',
            'options'     => ['false' => '禁用', 'true' => '启用'],
        ],
        'enable_lxcfs' => [
            'type'        => 'dropdown',
            'name'        => 'LXCFS资源视图',
            'description' => '显示真实资源限制',
            'default'     => 'true',
            'key'         => 'enable_lxcfs',
            'options'     => ['true' => '启用', 'false' => '禁用'],
        ],
    ];
}

function lxdserver_TestLink($params)
{
    lxdserver_debug('开始测试API连接', $params);

    $data = [
        'url'  => '/api/check',
        'type' => 'application/x-www-form-urlencoded',
        'data' => [],
    ];

    $res = lxdserver_Curl($params, $data, 'GET');


    if ($res === null) {
        return [
            'status' => 200,
            'data'   => [
                'server_status' => 0,
                'msg'           => "连接失败: 无法连接到服务器"
            ]
        ];
    } elseif (isset($res['error'])) {
        return [
            'status' => 200,
            'data'   => [
                'server_status' => 0,
                'msg'           => "连接失败: " . $res['error']
            ]
        ];
    } elseif (isset($res['code']) && $res['code'] == 200) {
        return [
            'status' => 200,
            'data'   => [
                'server_status' => 1,
                'msg'           => "连接成功"
            ]
        ];
    } elseif (isset($res['lxd_version'])) {
        return [
            'status' => 200,
            'data'   => [
                'server_status' => 1,
                'msg'           => "连接成功"
            ]
        ];
    } elseif (isset($res['code'])) {
        return [
            'status' => 200,
            'data'   => [
                'server_status' => 0,
                'msg'           => "连接失败: " . ($res['msg'] ?? '服务器返回错误')
            ]
        ];
    } else {
        return [
            'status' => 200,
            'data'   => [
                'server_status' => 0,
                'msg'           => "连接失败: 响应格式异常"
            ]
        ];
    }
}

function lxdserver_ClientArea($params)
{
    $pages = [
        'info'     => ['name' => '产品信息'],
    ];
    
    $network_mode = $params['configoptions']['network_mode'] ?? 'mode1';
    
    if (in_array($network_mode, ['mode1', 'mode2'])) {
        $pages['nat_acl'] = ['name' => 'NAT转发'];
    }
    
    if ($network_mode === 'mode2') {
        $pages['ipv6_acl'] = ['name' => 'IPv6绑定'];
    }
    
    $proxy_enabled = ($params['configoptions']['proxy_enabled'] ?? 'false') === 'true';
    if ($proxy_enabled) {
        $pages['proxy_acl'] = ['name' => '反向代理'];
    }
    
    return $pages;
}

function lxdserver_ClientAreaOutput($params, $key)
{
    lxdserver_debug('ClientAreaOutput调用', ['key' => $key, 'action' => $_GET['action'] ?? null]);

    if (isset($_GET['action'])) {
        $action = $_GET['action'];
        lxdserver_debug('处理API请求', ['action' => $action, 'domain' => $params['domain'] ?? null]);

        if (empty($params['domain'])) {
            header('Content-Type: application/json');
            echo json_encode(['code' => 400, 'msg' => '容器域名未设置']);
            exit;
        }

        if ($action === 'natcheck') {
            header('Content-Type: application/json');
            echo json_encode(lxdserver_natcheck($params));
            exit;
        }

        if ($action === 'proxycheck') {
            header('Content-Type: application/json');
            echo json_encode(lxdserver_proxycheck($params));
            exit;
        }

        $apiEndpoints = [
            'getinfo'    => '/api/status',
            'getstats'   => '/api/info',
            'gettraffic' => '/api/traffic',
            'getinfoall' => '/api/info',
            'natlist'    => '/api/natlist',
            'ipv6list'   => '/api/ipv6/list',
            'proxylist'  => '/api/proxy/list',
        ];

        $apiEndpoint = $apiEndpoints[$action] ?? '';

        if (!$apiEndpoint) {
            header('Content-Type: application/json');
            echo json_encode(['code' => 400, 'msg' => '不支持的操作: ' . $action]);
            exit;
        }

        $requestData = [
            'url'  => $apiEndpoint . '?hostname=' . $params['domain'],
            'type' => 'application/x-www-form-urlencoded',
            'data' => [],
        ];


        $res = lxdserver_Curl($params, $requestData, 'GET');


        if ($res === null) {
            $res = ['code' => 500, 'msg' => '连接服务器失败'];
        } elseif (!is_array($res)) {
            $res = ['code' => 500, 'msg' => '服务器返回了无效的响应格式'];
        } else {
            $res = lxdserver_TransformAPIResponse($action, $res);
        }

        header('Content-Type: application/json');
        header('Cache-Control: no-cache, no-store, must-revalidate');
        header('Pragma: no-cache');
        header('Expires: 0');
        echo json_encode($res);
        exit;
    }

    if ($key == 'info') {
        return [
            'template' => 'templates/info.html',
            'vars'     => [],
        ];
    }

    if ($key == 'nat_acl') {
        $network_mode = $params['configoptions']['network_mode'] ?? 'mode1';
        $nat_enabled = in_array($network_mode, ['mode1', 'mode2']);
        
        $requestData = [
            'url'  => '/api/natlist?hostname=' . $params['domain'] . '&_t=' . time(),
            'type' => 'application/x-www-form-urlencoded',
            'data' => [],
        ];
        $res = lxdserver_Curl($params, $requestData, 'GET');

        $nat_limit = intval($params['configoptions']['nat_limit'] ?? 5);
        $current_count = lxdserver_getNATRuleCount($params);
        $udp_enabled = ($params['configoptions']['udp_enabled'] ?? 'false') === 'true';

        return [
            'template' => 'templates/nat.html',
            'vars'     => [
                'list' => $res['data'] ?? [],
                'msg'  => $res['msg'] ?? '',
                'nat_limit' => $nat_limit,
                'current_count' => $current_count,
                'remaining_count' => max(0, $nat_limit - $current_count),
                'udp_enabled' => $udp_enabled,
                'nat_enabled' => $nat_enabled,
            ],
        ];
    }

    if ($key == 'ipv6_acl') {
        $network_mode = $params['configoptions']['network_mode'] ?? 'mode1';
        $ipv6_enabled = ($network_mode === 'mode2');
        
        $requestData = [
            'url'  => '/api/ipv6/list?hostname=' . $params['domain'] . '&_t=' . time(),
            'type' => 'application/x-www-form-urlencoded',
            'data' => [],
        ];
        $res = lxdserver_Curl($params, $requestData, 'GET');

        $ipv6_limit = intval($params['configoptions']['ipv6_limit'] ?? 1);
        $current_count = lxdserver_getIPv6BindingCount($params);
        $ipv6_allow_delete = ($params['configoptions']['ipv6_allow_delete'] ?? 'true') === 'true';

        return [
            'template' => 'templates/ipv6.html',
            'vars'     => [
                'list' => $res['data'] ?? [],
                'msg'  => $res['msg'] ?? '',
                'ipv6_limit' => $ipv6_limit,
                'current_count' => $current_count,
                'remaining_count' => max(0, $ipv6_limit - $current_count),
                'container_name' => $params['domain'],
                'ipv6_enabled' => $ipv6_enabled,
                'ipv6_allow_delete' => $ipv6_allow_delete,
            ],
        ];
    }
    
    if ($key == 'proxy_acl') {
        $proxy_enabled = ($params['configoptions']['proxy_enabled'] ?? 'false') === 'true';
        
        $requestData = [
            'url'  => '/api/proxy/list?hostname=' . $params['domain'] . '&_t=' . time(),
            'type' => 'application/x-www-form-urlencoded',
            'data' => [],
        ];
        
        $res = lxdserver_curl($params, $requestData);
        
        $proxy_limit = intval($params['configoptions']['proxy_limit'] ?? 1);
        $current_count = is_array($res['data']) ? count($res['data']) : 0;
        
        return [
            'template' => 'templates/proxy.html',
            'vars'     => [
                'list' => $res['data'] ?? [],
                'msg'  => $res['msg'] ?? '',
                'proxy_limit' => $proxy_limit,
                'current_count' => $current_count,
                'remaining_count' => max(0, $proxy_limit - $current_count),
                'container_name' => $params['domain'],
                'proxy_enabled' => $proxy_enabled,
            ],
        ];
    }
}

function lxdserver_getContainerIPs($params, $hostname) {
    $network_mode = $params['configoptions']['network_mode'] ?? 'mode1';
    $server_ipv4 = $params['server_ip'];
    
    $dedicatedip = '';
    $assignedips = '';
    
    switch ($network_mode) {
        case 'mode1':
            $dedicatedip = $server_ipv4;
            $assignedips = '';
            break;
        case 'mode2':
            $dedicatedip = $server_ipv4;
            $ipv6_list = lxdserver_getIndependentIPv6List($params);
            $assignedips = !empty($ipv6_list) ? $ipv6_list[0] : '';
            break;
    }
    
    return [
        'dedicatedip' => $dedicatedip,
        'assignedips' => $assignedips,
    ];
}

function lxdserver_getIndependentIPv6List($params)
{
    $data = [
        'url'  => '/api/ipv6/list?hostname=' . urlencode($params['domain']),
        'type' => 'application/x-www-form-urlencoded',
        'data' => [],
    ];

    $res = lxdserver_Curl($params, $data, 'GET');

    if (isset($res['code']) && $res['code'] == 200 && isset($res['data']) && is_array($res['data'])) {
        $ipv6_addresses = [];
        foreach ($res['data'] as $item) {
            if (isset($item['public_ipv6'])) {
                $ipv6_addresses[] = $item['public_ipv6'];
            }
        }
        return $ipv6_addresses;
    }

    return [];
}

function lxdserver_AllowFunction()
{
    return [
        'client' => ['natadd', 'natdel', 'natlist', 'natcheck', 'ipv6add', 'ipv6del', 'ipv6list', 'proxyadd', 'proxydel', 'proxylist', 'proxycheck'],
    ];
}

function lxdserver_CreateAccount($params)
{
    lxdserver_debug('开始创建容器', ['domain' => $params['domain']]);

    $sys_pwd = $params['password'] ?? randStr(8);

    $data = [
        'url'  => '/api/create',
        'type' => 'application/json',
        'data' => [
            'hostname'      => $params['domain'],
            'password'      => $sys_pwd,
            'cpus'          => (int)($params['configoptions']['cpus'] ?? 1),
            'memory'        => $params['configoptions']['memory'] ?? '512MB',
            'disk'          => $params['configoptions']['disk'] ?? '10GB',
            'image'         => $params['configoptions']['image'] ?? 'ubuntu24',
            'ingress'       => $params['configoptions']['ingress'] ?? '100Mbit',
            'egress'        => $params['configoptions']['egress'] ?? '100Mbit',
            'allow_nesting' => ($params['configoptions']['allow_nesting'] ?? 'false') === 'true',
            'traffic_limit' => (int)($params['configoptions']['traffic_limit'] ?? 0),
            'network_mode'  => $params['configoptions']['network_mode'] ?? 'mode1',
            'cpu_allowance'  => $params['configoptions']['cpu_allowance'] ?? '100%',
            'memory_swap'           => ($params['configoptions']['memory_swap'] ?? 'true') === 'true',
            'max_processes'  => (int)($params['configoptions']['max_processes'] ?? 512),
            'disk_io_limit'   => $params['configoptions']['disk_io_limit'] ?? '',
            'privileged'     => ($params['configoptions']['privileged'] ?? 'false') === 'true',
            'enable_lxcfs'   => ($params['configoptions']['enable_lxcfs'] ?? 'true') === 'true',
        ],
    ];

    lxdserver_debug('发送创建请求', $data);
    $res = lxdserver_JSONCurl($params, $data, 'POST');
    lxdserver_debug('创建响应', $res);

    if (isset($res['code']) && $res['code'] == '200') {
        sleep(2);
        
        $dedicatedip = '';
        $assignedips = '';
        
        if (!empty($res['data']['dedicatedip'])) {
            $dedicatedip = $res['data']['dedicatedip'];
            lxdserver_debug('从创建响应获取到dedicatedip', ['dedicatedip' => $dedicatedip]);
        }
        
        if (!empty($res['data']['assignedips'])) {
            $assignedips = $res['data']['assignedips'];
            lxdserver_debug('从创建响应获取到assignedips', ['assignedips' => $assignedips]);
        }

        $update = [
            'dedicatedip'  => $dedicatedip,
            'assignedips'  => $assignedips,
            'domainstatus' => 'Active',
            'username'     => 'root',
        ];

        try {
            Db::name('host')->where('id', $params['hostid'])->update($update);
            lxdserver_debug('数据库更新成功', $update);
        } catch (\Exception $e) {
             return ['status' => 'error', 'msg' => ($res['msg'] ?? '创建成功，但同步数据到面板失败: ' . $e->getMessage())];
        }

        return ['status' => 'success', 'msg' => $res['msg'] ?? '创建成功'];
    } else {
        return ['status' => 'error', 'msg' => $res['msg'] ?? '创建失败'];
    }
}

function lxdserver_Sync($params)
{
    $data = [
        'url'  => '/api/status?hostname=' . $params['domain'],
        'type' => 'application/x-www-form-urlencoded',
        'data' => [],
    ];
    $res = lxdserver_Curl($params, $data, 'GET');

    if (isset($res['code']) && $res['code'] == '200') {
        if (class_exists('think\Db') && isset($params['hostid'])) {
            try {
                $ipInfo = lxdserver_getContainerIPs($params, $params['domain']);
                
                $update_data = [
                    'dedicatedip' => $ipInfo['dedicatedip'],
                    'assignedips' => $ipInfo['assignedips'],
                ];

                Db::name('host')->where('id', $params['hostid'])->update($update_data);
            } catch (Exception $e) {
                lxdserver_debug('同步数据库失败', ['error' => $e->getMessage()]);
            }
        }
        return ['status' => 'success', 'msg' => $res['msg'] ?? '同步成功'];
    }

    return ['status' => 'error', 'msg' => $res['msg'] ?? '同步失败'];
}


function lxdserver_TerminateAccount($params)
{
    $data = [
        'url'  => '/api/delete?hostname=' . $params['domain'],
        'type' => 'application/x-www-form-urlencoded',
        'data' => [],
    ];
    $res = lxdserver_Curl($params, $data, 'GET');

    return isset($res['code']) && $res['code'] == '200'
        ? ['status' => 'success', 'msg' => $res['msg'] ?? '删除成功']
        : ['status' => 'error', 'msg' => $res['msg'] ?? '删除失败'];
}

function lxdserver_On($params)
{
    $data = [
        'url'  => '/api/boot?hostname=' . $params['domain'],
        'type' => 'application/x-www-form-urlencoded',
        'data' => [],
    ];
    $res = lxdserver_Curl($params, $data, 'GET');

    return isset($res['code']) && $res['code'] == '200'
        ? ['status' => 'success', 'msg' => $res['msg'] ?? '开机成功']
        : ['status' => 'error', 'msg' => $res['msg'] ?? '开机失败'];
}

function lxdserver_Off($params)
{
    $data = [
        'url'  => '/api/stop?' . 'hostname=' . $params['domain'],
        'type' => 'application/x-www-form-urlencoded',
        'data' => [],
    ];
    $res = lxdserver_Curl($params, $data, 'GET');

    if (isset($res['code']) && $res['code'] == '200') {
        return ['status' => 'success', 'msg' => $res['msg'] ?? '关机成功'];
    } else {
        return ['status' => 'error', 'msg' => $res['msg'] ?? '关机失败'];
    }
}

function lxdserver_SuspendAccount($params)
{
    lxdserver_debug('开始暂停容器', ['domain' => $params['domain']]);

    $data = [
        'url'  => '/api/suspend?hostname=' . $params['domain'],
        'type' => 'application/x-www-form-urlencoded',
        'data' => [],
    ];
    $res = lxdserver_Curl($params, $data, 'GET');



    if (isset($res['code']) && $res['code'] == '200') {
        return ['status' => 'success', 'msg' => $res['msg'] ?? '容器暂停任务已提交'];
    } else {
        return ['status' => 'error', 'msg' => $res['msg'] ?? '容器暂停失败'];
    }
}

function lxdserver_UnsuspendAccount($params)
{
    lxdserver_debug('开始解除暂停容器', ['domain' => $params['domain']]);

    $data = [
        'url'  => '/api/unsuspend?hostname=' . $params['domain'],
        'type' => 'application/x-www-form-urlencoded',
        'data' => [],
    ];
    $res = lxdserver_Curl($params, $data, 'GET');



    if (isset($res['code']) && $res['code'] == '200') {
        return ['status' => 'success', 'msg' => $res['msg'] ?? '容器恢复任务已提交'];
    } else {
        return ['status' => 'error', 'msg' => $res['msg'] ?? '容器恢复失败'];
    }
}

function lxdserver_Reboot($params)
{
    $data = [
        'url'  => '/api/reboot?' . 'hostname=' . $params['domain'],
        'type' => 'application/x-www-form-urlencoded',
        'data' => [],
    ];
    $res = lxdserver_Curl($params, $data, 'GET');

    if (isset($res['code']) && $res['code'] == '200') {
        return ['status' => 'success', 'msg' => $res['msg'] ?? '重启成功'];
    } else {
        return ['status' => 'error', 'msg' => $res['msg'] ?? '重启失败'];
    }
}

function lxdserver_getNATRuleCount($params)
{
    $data = [
        'url'  => '/api/natlist?hostname=' . urlencode($params['domain']),
        'type' => 'application/x-www-form-urlencoded',
        'data' => [],
    ];

    $res = lxdserver_Curl($params, $data, 'GET');

    if (isset($res['code']) && $res['code'] == 200 && isset($res['data']) && is_array($res['data'])) {
        $rules = $res['data'];
        
        $udp_enabled = ($params['configoptions']['udp_enabled'] ?? 'false') === 'true';
        $counted = [];
        
        foreach ($rules as $rule) {
            $external_port = $rule['external_port'] ?? $rule['dport'] ?? '';
            $internal_port = $rule['internal_port'] ?? $rule['sport'] ?? '';
            $external_port_end = $rule['external_port_end'] ?? $rule['dport_end'] ?? 0;
            $internal_port_end = $rule['internal_port_end'] ?? $rule['sport_end'] ?? 0;
            $protocol = strtolower($rule['protocol'] ?? $rule['dtype'] ?? '');
            
            $key = $external_port . '_' . $internal_port . '_' . $external_port_end . '_' . $internal_port_end;
            
            if (!isset($counted[$key])) {
                $counted[$key] = [
                    'external_port' => $external_port,
                    'external_port_end' => $external_port_end,
                    'protocols' => []
                ];
            }
            
            $counted[$key]['protocols'][] = $protocol;
        }
        
        $totalCount = 0;
        foreach ($counted as $item) {
            if ($item['external_port_end'] > 0) {
                $portCount = $item['external_port_end'] - $item['external_port'] + 1;
                $totalCount += $portCount;
            } else {
                $totalCount += 1;
            }
        }
        
        return $totalCount;
    }

    return 0;
}

function lxdserver_natadd($params)
{
    $network_mode = $params['configoptions']['network_mode'] ?? 'mode1';
    if (!in_array($network_mode, ['mode1', 'mode2'])) {
        return ['status' => 'error', 'msg' => 'NAT端口转发功能未启用，请联系管理员配置正确的网络模式。'];
    }
    
    parse_str(file_get_contents("php://input"), $post);

    $sport = intval($post['sport'] ?? 0);
    $dport = intval($post['dport'] ?? 0);
    $sport_end = intval($post['sport_end'] ?? 0);
    $dport_end = intval($post['dport_end'] ?? 0);
    $description = trim($post['description'] ?? '');
    $udp_enabled = ($params['configoptions']['udp_enabled'] ?? 'false') === 'true';

    $dtype = $udp_enabled ? 'both' : 'tcp';

    $nat_limit = intval($params['configoptions']['nat_limit'] ?? 5);
    $current_count = lxdserver_getNATRuleCount($params);
    
    if ($sport <= 0 || $sport > 65535) {
        return ['status' => 'error', 'msg' => '内网起始端口超过范围'];
    }
    
    $isPortRange = $sport_end > 0 && $dport_end > 0;
    
    if ($isPortRange) {
        if ($sport > $sport_end) {
            return ['status' => 'error', 'msg' => '内网端口范围错误'];
        }
        
        if ($dport > 0 && $dport > $dport_end) {
            return ['status' => 'error', 'msg' => '外网端口范围错误'];
        }
        
        $internal_range = $sport_end - $sport + 1;
        $external_range = $dport_end - $dport + 1;
        
        if ($internal_range !== $external_range) {
            return ['status' => 'error', 'msg' => '内网和外网端口数量必须一致'];
        }
        
        if ($current_count + $internal_range > $nat_limit) {
            return ['status' => 'error', 'msg' => "端口段包含 {$internal_range} 个端口，将超过NAT规则限制（剩余配额：" . ($nat_limit - $current_count) . "）"];
        }
        
        $requestData = 'hostname=' . urlencode($params['domain']) . 
                       '&dtype=' . urlencode($dtype) . 
                       '&sport=' . $sport . 
                       '&sport_end=' . $sport_end . 
                       '&dport=' . $dport . 
                       '&dport_end=' . $dport_end;
        
        if (!empty($description)) {
            $requestData .= '&description=' . urlencode($description);
        }
        
        $data = [
            'url'  => '/api/addport',
            'type' => 'application/x-www-form-urlencoded',
            'data' => $requestData,
        ];

        $res = lxdserver_Curl($params, $data, 'POST');

        $protocol_desc = $udp_enabled ? 'TCP+UDP双协议' : 'TCP';
        if (isset($res['code']) && $res['code'] == 200) {
            return ['status' => 'success', 'msg' => "端口段添加成功（{$internal_range}个端口，{$protocol_desc}）"];
        } else {
            return ['status' => 'error', 'msg' => $res['msg'] ?? '端口段添加失败'];
        }
    }
    
    if ($current_count >= $nat_limit) {
        return ['status' => 'error', 'msg' => "NAT规则数量已达到限制（{$nat_limit}条），无法添加更多规则"];
    }

    $requestData = 'hostname=' . urlencode($params['domain']) . '&dtype=' . urlencode($dtype) . '&sport=' . $sport;

    if ($dport > 0) {
        if ($dport < 10000 || $dport > 65535) {
            return ['status' => 'error', 'msg' => '外网端口范围为10000-65535'];
        }
        $checkData = [
            'url'  => '/api/nat/check?hostname=' . urlencode($params['domain']) . '&protocol=' . urlencode($dtype) . '&port=' . $dport,
            'type' => 'application/x-www-form-urlencoded',
            'data' => [],
        ];
        $checkRes = lxdserver_Curl($params, $checkData, 'GET');
        if (!isset($checkRes['code']) || $checkRes['code'] != 200 || empty($checkRes['data']['available'])) {
            $reason = $checkRes['data']['reason'] ?? $checkRes['msg'] ?? '端口不可用';
            return ['status' => 'error', 'msg' => $reason];
        }
        $requestData .= '&dport=' . $dport;
    }
    
    if (!empty($description)) {
        $requestData .= '&description=' . urlencode($description);
    }

    $data = [
        'url'  => '/api/addport',
        'type' => 'application/x-www-form-urlencoded',
        'data' => $requestData,
    ];

    $res = lxdserver_Curl($params, $data, 'POST');

    $protocol_desc = $udp_enabled ? 'TCP+UDP双协议' : 'TCP';
    if (isset($res['code']) && $res['code'] == 200) {
        return ['status' => 'success', 'msg' => "NAT转发添加成功（{$protocol_desc}）"];
    } else {
        return ['status' => 'error', 'msg' => $res['msg'] ?? 'NAT转发添加失败'];
    }
}

function lxdserver_natdel($params)
{
    
    parse_str(file_get_contents("php://input"), $post);

    $dport = intval($post['dport'] ?? 0);
    $sport = intval($post['sport'] ?? 0);
    $dport_end = intval($post['dport_end'] ?? 0);
    $sport_end = intval($post['sport_end'] ?? 0);
    $dtype = strtolower(trim($post['dtype'] ?? ''));
    $udp_enabled = ($params['configoptions']['udp_enabled'] ?? 'false') === 'true';

    if (!in_array($dtype, ['tcp', 'udp', 'both'])) {
        return ['status' => 'error', 'msg' => '不支持的协议类型'];
    }
    
    if ($dtype === 'udp' && !$udp_enabled) {
    }
    if ($sport <= 0 || $sport > 65535) {
        return ['status' => 'error', 'msg' => '容器内部端口超过范围'];
    }
    if ($dport < 10000 || $dport > 65535) {
        return ['status' => 'error', 'msg' => '外网端口映射范围为10000-65535'];
    }

    if ($dtype === 'both') {
        $success_count = 0;
        $error_msgs = [];
        
        foreach (['tcp', 'udp'] as $protocol) {
            $requestData = 'hostname=' . urlencode($params['domain']) . '&dtype=' . urlencode($protocol) . '&dport=' . $dport . '&sport=' . $sport;
            
            if ($dport_end > 0 && $sport_end > 0) {
                $requestData .= '&dport_end=' . $dport_end . '&sport_end=' . $sport_end;
            }
            
            $data = [
                'url'  => '/api/delport',
                'type' => 'application/x-www-form-urlencoded',
                'data' => $requestData,
            ];
            
            $res = lxdserver_Curl($params, $data, 'POST');
            
            if (isset($res['code']) && $res['code'] == 200) {
                $success_count++;
            } else {
                $error_msgs[] = $protocol . ': ' . ($res['msg'] ?? '删除失败');
            }
        }
        
        if ($success_count === 2) {
            return ['status' => 'success', 'msg' => 'NAT转发删除成功'];
        } else if ($success_count === 1) {
            return ['status' => 'success', 'msg' => 'NAT转发部分删除成功：' . implode(', ', $error_msgs)];
        } else {
            return ['status' => 'error', 'msg' => 'NAT转发删除失败：' . implode(', ', $error_msgs)];
        }
    }

    $requestData = 'hostname=' . urlencode($params['domain']) . '&dtype=' . urlencode($dtype) . '&dport=' . $dport . '&sport=' . $sport;
    
    if ($dport_end > 0 && $sport_end > 0) {
        $requestData .= '&dport_end=' . $dport_end . '&sport_end=' . $sport_end;
    }
    
    $data = [
        'url'  => '/api/delport',
        'type' => 'application/x-www-form-urlencoded',
        'data' => $requestData,
    ];

    $res = lxdserver_Curl($params, $data, 'POST');

    if (isset($res['code']) && $res['code'] == 200) {
        return ['status' => 'success', 'msg' => $res['msg'] ?? 'NAT转发删除成功'];
    } else {
        return ['status' => 'error', 'msg' => $res['msg'] ?? 'NAT转发删除失败'];
    }
}

function lxdserver_Status($params)
{
    $data = [
        'url'  => '/api/status?' . 'hostname=' . $params['domain'],
        'type' => 'application/x-www-form-urlencoded',
        'data' => [],
    ];
    $res = lxdserver_Curl($params, $data, 'GET');

    if (isset($res['code']) && $res['code'] == 200) {
        $result = ['status' => 'success'];

        $containerStatus = $res['data']['status'] ?? '';

        switch (strtoupper($containerStatus)) {
            case 'RUNNING':
                $result['data']['status'] = 'on';
                $result['data']['des'] = '开机';
                break;
            case 'STOPPED':
                $result['data']['status'] = 'off';
                $result['data']['des'] = '关机';
                break;
            case 'FROZEN':
                $result['data']['status'] = 'suspend';
                $result['data']['des'] = '流量超标-暂停';
                break;
            default:
                $result['data']['status'] = 'unknown';
                $result['data']['des'] = '未知状态';
                break;
        }

        return $result;
    } else {
        return ['status' => 'error', 'msg' => $res['msg'] ?? '获取状态失败'];
    }
}

function lxdserver_CrackPassword($params, $new_pass)
{
    $data = [
        'url'  => '/api/password',
        'type' => 'application/json',
        'data' => [
            'hostname' => $params['domain'],
            'password' => $new_pass,
        ],
    ];
    $res = lxdserver_JSONCurl($params, $data, 'POST');

    if (isset($res['code']) && $res['code'] == 200) {
        try {
            Db::name('host')->where('id', $params['hostid'])->update(['password' => $new_pass]);
        } catch (\Exception $e) {
            return ['status' => 'error', 'msg' => ($res['msg'] ?? $res['message'] ?? 'LXD容器密码重置成功，但同步新密码到面板数据库失败: ' . $e->getMessage())];
        }
        return ['status' => 'success', 'msg' => $res['msg'] ?? $res['message'] ?? '密码重置成功'];
    } else {
        return ['status' => 'error', 'msg' => $res['msg'] ?? $res['message'] ?? '密码重置失败'];
    }
}

function lxdserver_Reinstall($params)
{
    if (empty($params['reinstall_os'])) {
        return ['status' => 'error', 'msg' => '操作系统参数错误'];
    }

    $reinstall_pass = $params['password'] ?? randStr(8);

    $data = [
        'url'  => '/api/reinstall',
        'type' => 'application/json',
        'data' => [
            'hostname' => $params['domain'],
            'system'   => $params['reinstall_os'],
            'password' => $reinstall_pass,

            'cpus'          => (int)($params['configoptions']['cpus'] ?? 1),
            'memory'        => $params['configoptions']['memory'] ?? '512MB',
            'disk'          => $params['configoptions']['disk'] ?? '10GB',
            'ingress'       => $params['configoptions']['ingress'] ?? '100Mbit',
            'egress'        => $params['configoptions']['egress'] ?? '100Mbit',
            'allow_nesting' => ($params['configoptions']['allow_nesting'] ?? 'false') === 'true',
            'traffic_limit' => (int)($params['configoptions']['traffic_limit'] ?? 0),
            'cpu_allowance'  => $params['configoptions']['cpu_allowance'] ?? '100%',
            'memory_swap'           => ($params['configoptions']['memory_swap'] ?? 'true') === 'true',
            'max_processes'  => (int)($params['configoptions']['max_processes'] ?? 512),
            'disk_io_limit'   => $params['configoptions']['disk_io_limit'] ?? '',
            'privileged'     => ($params['configoptions']['privileged'] ?? 'false') === 'true',
            'enable_lxcfs'   => ($params['configoptions']['enable_lxcfs'] ?? 'true') === 'true',
        ],
    ];
    $res = lxdserver_JSONCurl($params, $data, 'POST');

    if (isset($res['code']) && $res['code'] == 200) {
        return ['status' => 'success', 'msg' => $res['msg'] ?? $res['message'] ?? '重装成功'];
    } else {
        return ['status' => 'error', 'msg' => $res['msg'] ?? $res['message'] ?? '重装失败'];
    }
}

function lxdserver_JSONCurl($params, $data = [], $request = 'POST')
{
    $curl = curl_init();

    $protocol = 'https';
    $url = $protocol . '://' . $params['server_ip'] . ':' . $params['port'] . $data['url'];

    $curlOptions = [
        CURLOPT_URL            => $url,
        CURLOPT_RETURNTRANSFER => true,
        CURLOPT_ENCODING       => '',
        CURLOPT_MAXREDIRS      => 10,
        CURLOPT_TIMEOUT        => 30,
        CURLOPT_CONNECTTIMEOUT => 10,
        CURLOPT_FOLLOWLOCATION => true,
        CURLOPT_HTTP_VERSION   => CURL_HTTP_VERSION_1_1,
        CURLOPT_CUSTOMREQUEST  => $request,
        CURLOPT_POSTFIELDS     => json_encode($data['data']),
        CURLOPT_HTTPHEADER     => [
            'apikey: ' . $params['accesshash'],
            'Content-Type: application/json',
        ],
    ];

    $curlOptions[CURLOPT_SSL_VERIFYPEER] = false;
    $curlOptions[CURLOPT_SSL_VERIFYHOST] = false;
    $curlOptions[CURLOPT_SSLVERSION] = CURL_SSLVERSION_TLSv1_2;

    curl_setopt_array($curl, $curlOptions);

    $response = curl_exec($curl);
    $errno    = curl_errno($curl);

    curl_close($curl);

    if ($errno) {
        return null;
    }

    return json_decode($response, true);
}

function lxdserver_Curl($params, $data = [], $request = 'POST')
{
    $curl = curl_init();

    $protocol = 'https';
    $url = $protocol . '://' . $params['server_ip'] . ':' . $params['port'] . $data['url'];

    lxdserver_debug('发送请求', [
        'url' => $url,
        'method' => $request
    ]);

    $postFields = ($request === 'POST' || $request === 'PUT') ? ($data['data'] ?? null) : null;
    if ($request === 'GET' && !empty($data['data']) && is_array($data['data'])) {
        $url .= (strpos($url, '?') === false ? '?' : '&') . http_build_query($data['data']);
    } elseif ($request === 'GET' && !empty($data['data']) && is_string($data['data'])) {
         $url .= (strpos($url, '?') === false ? '?' : '&') . $data['data'];
    }

    $curlOptions = [
        CURLOPT_URL            => $url,
        CURLOPT_RETURNTRANSFER => true,
        CURLOPT_ENCODING       => '',
        CURLOPT_MAXREDIRS      => 10,
        CURLOPT_TIMEOUT        => 30,
        CURLOPT_CONNECTTIMEOUT => 10,
        CURLOPT_FOLLOWLOCATION => true,
        CURLOPT_HTTP_VERSION   => CURL_HTTP_VERSION_1_1,
        CURLOPT_CUSTOMREQUEST  => $request,
        CURLOPT_HTTPHEADER     => [
            'apikey: ' . $params['accesshash'],
            'Content-Type: ' . ($data['type'] ?? 'application/x-www-form-urlencoded'),
        ],
    ];

    $curlOptions[CURLOPT_SSL_VERIFYPEER] = false;
    $curlOptions[CURLOPT_SSL_VERIFYHOST] = false;
    $curlOptions[CURLOPT_SSLVERSION] = CURL_SSLVERSION_TLSv1_2;

    curl_setopt_array($curl, $curlOptions);

    if ($postFields !== null) {
        curl_setopt($curl, CURLOPT_POSTFIELDS, $postFields);
    }

    $response = curl_exec($curl);
    $errno    = curl_errno($curl);
    $httpCode = curl_getinfo($curl, CURLINFO_HTTP_CODE);
    $curlError = curl_error($curl);

    curl_close($curl);

    lxdserver_debug('请求响应', [
        'http_code' => $httpCode,
        'response_length' => strlen($response),
        'curl_errno' => $errno,
        'curl_error' => $curlError
    ]);

    if ($errno) {
        lxdserver_debug('CURL错误', [
            'errno' => $errno,
            'error' => $curlError,
            'error_desc' => curl_strerror($errno)
        ]);
        return null;
    }

    $decoded = json_decode($response, true);
    lxdserver_debug('解析响应', ['code' => $decoded['code'] ?? 'NO CODE']);
    return $decoded;
}

function lxdserver_TransformAPIResponse($action, $response)
{
    if (isset($response['error'])) {
        return [
            'code' => 400,
            'msg' => $response['error']
        ];
    }

    if (!isset($response['code']) || $response['code'] != 200) {
        return $response; 
    }

    switch ($action) {
        case 'getinfo':
            return $response;

        case 'getstats':
        case 'getinfoall':
            if (isset($response['data'])) {
                $data = $response['data'];

                $transformed = [
                    'code' => 200,
                    'msg' => '获取容器信息成功',
                    'data' => [
                        'hostname' => $data['hostname'] ?? '',
                        'status' => $data['status'] ?? '',
                        'ipv4' => $data['ipv4'] ?? '',
                        'ipv6' => $data['ipv6'] ?? '',
                        'type' => $data['type'] ?? '',
                        'created_at' => $data['created_at'] ?? '',
                        'cpus' => $data['config']['cpus'] ?? 1,
                        'memory' => $data['memory'] ?? 1024,
                        'disk' => $data['disk'] ?? 10240,
                        'config' => [
                            'cpus' => $data['config']['cpus'] ?? 1,
                            'memory' => $data['config']['memory'] ?? '1024 MB',
                            'disk' => $data['config']['disk'] ?? '10240 MB',
                            'traffic_limit' => $data['config']['traffic_limit'] ?? 0,
                        ],
                        'cpu_usage' => $data['cpu_usage'] ?? 0,
                        'memory_usage' => $data['memory_usage'] ?? '0 B',
                        'memory_usage_raw' => $data['memory_usage_raw'] ?? 0,
                        'disk_usage' => $data['disk_usage'] ?? '0 B',
                        'disk_usage_raw' => $data['disk_usage_raw'] ?? 0,
                        'traffic_usage' => $data['traffic_usage'] ?? '0 B',
                        'traffic_usage_raw' => $data['traffic_usage_raw'] ?? 0,
                        'cpu_percent' => $data['cpu_percent'] ?? 0,
                        'memory_percent' => $data['memory_percent'] ?? 0,
                        'disk_percent' => $data['disk_percent'] ?? 0,
                        'last_update' => date('Y-m-d H:i:s'),
                        'timestamp' => time(),
                    ]
                ];

                return $transformed;
            }
            break;

        case 'gettraffic':
            if (isset($response['data']['used'])) {
                return [
                    'code' => 200,
                    'msg' => '获取流量使用量成功',
                    'data' => [
                        'used' => $response['data']['used'],
                    ]
                ];
            }
            break;
        
        case 'ipv6list':
            if (isset($response['data']) && is_array($response['data'])) {
                return [
                    'code' => 200,
                    'msg' => $response['msg'] ?? 'IPv6列表获取成功',
                    'data' => [
                        'list' => $response['data'],
                        'limit' => 0,
                        'current' => count($response['data']),
                    ]
                ];
            }
            break;
        
        case 'natlist':
            if (isset($response['data']) && is_array($response['data'])) {
                return [
                    'code' => 200,
                    'msg' => $response['msg'] ?? 'NAT列表获取成功',
                    'data' => [
                        'list' => $response['data'],
                        'limit' => 0,
                        'current' => count($response['data']),
                    ]
                ];
            }
            break;
        
        case 'proxylist':
            if (isset($response['data']) && is_array($response['data'])) {
                return [
                    'code' => 200,
                    'msg' => $response['msg'] ?? 'Proxy列表获取成功',
                    'data' => [
                        'list' => $response['data'],
                        'limit' => 0,
                        'current' => count($response['data']),
                    ]
                ];
            }
            break;
    }

    return $response;
}

function lxdserver_natlist($params)
{
    
    $requestData = [
        'url'  => '/api/natlist?' . 'hostname=' . $params['domain'] . '&_t=' . time(),
        'type' => 'application/x-www-form-urlencoded',
        'data' => [],
    ];
    $res = lxdserver_Curl($params, $requestData, 'GET');
    if ($res === null) {
        return ['code' => 500, 'msg' => '连接API服务器失败', 'data' => []];
    }
    return $res;
}

function lxdserver_natcheck($params)
{
    $dport = intval($_GET['dport'] ?? 0);
    $dtype = strtolower(trim($_GET['dtype'] ?? ''));
    $hostname = trim($_GET['hostname'] ?? '');

    if ($dport <= 0) {
        $dport = intval($_POST['dport'] ?? 0);
    }
    if (empty($dtype)) {
        $dtype = strtolower(trim($_POST['dtype'] ?? 'tcp'));
    }
    if (empty($hostname)) {
        $hostname = trim($_POST['hostname'] ?? '');
    }

    if ($dport <= 0 || empty($hostname)) {
        $postRaw = file_get_contents("php://input");
        if (!empty($postRaw)) {
            parse_str($postRaw, $input);
            if ($dport <= 0) {
                $dport = intval($input['dport'] ?? 0);
            }
            if (empty($dtype)) {
                $dtype = strtolower(trim($input['dtype'] ?? 'tcp'));
            }
            if (empty($hostname)) {
                $hostname = trim($input['hostname'] ?? '');
            }
        }
    }

    if (empty($hostname)) {
        $hostname = trim($params['domain'] ?? '');
    }

    lxdserver_debug('natcheck参数解析', [
        'dport' => $dport, 
        'dtype' => $dtype, 
        'hostname' => $hostname,
        'GET' => $_GET,
        'POST' => $_POST,
        'raw_input' => file_get_contents("php://input"),
        'params_domain' => $params['domain'] ?? 'null'
    ]);

    if ($dport <= 0) {
        return ['code' => 400, 'msg' => '缺少端口参数', 'data' => ['available' => false, 'reason' => '缺少端口参数']];
    }
    if (!in_array($dtype, ['tcp', 'udp'])) {
        return ['code' => 400, 'msg' => '协议类型错误', 'data' => ['available' => false, 'reason' => '协议类型错误']];
    }
    if ($dport < 10000 || $dport > 65535) {
        return ['code' => 400, 'msg' => '端口范围为10000-65535', 'data' => ['available' => false, 'reason' => '端口范围为10000-65535']];
    }
    if (empty($hostname)) {
        return ['code' => 400, 'msg' => '容器标识缺失', 'data' => ['available' => false, 'reason' => '容器标识缺失']];
    }

    $queryParams = http_build_query([
        'hostname' => $hostname,
        'protocol' => $dtype,
        'port'     => $dport,
    ]);

    $requestData = [
        'url'  => '/api/nat/check?' . $queryParams,
        'type' => 'application/x-www-form-urlencoded',
        'data' => '',
    ];

    $res = lxdserver_Curl($params, $requestData, 'GET');

    if ($res === null) {
        return ['code' => 500, 'msg' => '连接服务器失败', 'data' => ['available' => false, 'reason' => '连接服务器失败']];
    }

    if (!isset($res['code'])) {
        return ['code' => 500, 'msg' => '服务器返回无效数据', 'data' => ['available' => false, 'reason' => '服务器返回无效数据']];
    }

    return $res;
}

function lxdserver_vnc($params) {
    lxdserver_debug('VNC控制台请求', ['domain' => $params['domain']]);

    $data = [
        'url'  => '/api/status?hostname=' . $params['domain'],
        'type' => 'application/x-www-form-urlencoded',
        'data' => [],
    ];
    $res = lxdserver_Curl($params, $data, 'GET');

    if (!isset($res['code']) || $res['code'] != '200') {
        return ['status' => 'error', 'msg' => $res['msg'] ?? '无法获取容器状态'];
    }

    if (!isset($res['data']['status']) || $res['data']['status'] !== 'RUNNING') {
        return ['status' => 'error', 'msg' => '容器未运行，无法连接控制台'];
    }

    $tokenData = [
        'url'  => '/api/console/create-token',
        'type' => 'application/json',
        'data' => [
            'hostname' => $params['domain'],
            'user_id' => (int)($params['userid'] ?? 0),
            'service_id' => (int)($params['serviceid'] ?? 0),
            'server_ip' => $params['server_ip'] ?? '',
            'expires_in' => 3600
        ],
    ];

    $tokenRes = lxdserver_JSONCurl($params, $tokenData, 'POST');


    if (!isset($tokenRes['code']) || $tokenRes['code'] != 200) {
        return ['status' => 'error', 'msg' => $tokenRes['msg'] ?? '生成控制台令牌失败'];
    }

    $protocol = 'https';
    $consoleUrl = $protocol . '://' . $params['server_ip'] . ':' . $params['port'] . '/console?token=' . $tokenRes['data']['token'];



    return [
        'status' => 'success',
        'url' => $consoleUrl,
        'msg' => '控制台连接已准备就绪'
    ];
}


function lxdserver_AdminButton($params)
{
    if (!empty($params['domain'])) {
        return [
            'TrafficReset' => '流量重置',
        ];
    }
    return [];
}

function lxdserver_TrafficReset($params)
{
    lxdserver_debug('流量重置请求', ['domain' => $params['domain']]);

    if (empty($params['domain'])) {
        return ['status' => 'error', 'msg' => '容器域名参数缺失'];
    }

    $data = [
        'url'  => '/api/traffic/reset?hostname=' . urlencode($params['domain']),
        'type' => 'application/x-www-form-urlencoded',
        'data' => [],
    ];

    $res = lxdserver_Curl($params, $data, 'POST');

    if (isset($res['code']) && $res['code'] == 200) {
        return ['status' => 'success', 'msg' => $res['msg'] ?? '流量统计已重置'];
    } else {
        return ['status' => 'error', 'msg' => $res['msg'] ?? '流量重置失败'];
    }
}

function lxdserver_getIPv6BindingCount($params)
{
    $data = [
        'url'  => '/api/ipv6/list?hostname=' . urlencode($params['domain']),
        'type' => 'application/x-www-form-urlencoded',
        'data' => [],
    ];

    $res = lxdserver_Curl($params, $data, 'GET');

    if (isset($res['code']) && $res['code'] == 200 && isset($res['data']) && is_array($res['data'])) {
        return count($res['data']);
    }

    return 0;
}

function lxdserver_ipv6add($params)
{
    $network_mode = $params['configoptions']['network_mode'] ?? 'mode1';
    if ($network_mode !== 'mode2') {
        return ['status' => 'error', 'msg' => 'IPv6独立绑定功能未启用，请联系管理员配置为模式2。'];
    }
    
    parse_str(file_get_contents("php://input"), $post);

    $description = trim($post['description'] ?? '');

    $ipv6_limit = intval($params['configoptions']['ipv6_limit'] ?? 1);
    $current_count = lxdserver_getIPv6BindingCount($params);
    
    if ($current_count >= $ipv6_limit) {
        return ['status' => 'error', 'msg' => "IPv6绑定数量已达到限制（{$ipv6_limit}个），无法添加更多绑定"];
    }

    $requestData = 'hostname=' . urlencode($params['domain']) . '&description=' . urlencode($description);

    $data = [
        'url'  => '/api/ipv6/add',
        'type' => 'application/x-www-form-urlencoded',
        'data' => $requestData,
    ];

    $res = lxdserver_Curl($params, $data, 'POST');

    if (isset($res['code']) && $res['code'] == 200) {
        return ['status' => 'success', 'msg' => $res['msg'] ?? 'IPv6绑定添加成功'];
    } else {
        return ['status' => 'error', 'msg' => $res['msg'] ?? 'IPv6绑定添加失败'];
    }
}

function lxdserver_ipv6del($params)
{
    $ipv6_allow_delete = ($params['configoptions']['ipv6_allow_delete'] ?? 'true') === 'true';
    if (!$ipv6_allow_delete) {
        return ['status' => 'error', 'msg' => '管理员已禁止删除IPv6地址，如需更换IP请联系管理员处理'];
    }
    
    parse_str(file_get_contents("php://input"), $post);

    $public_ipv6 = trim($post['public_ipv6'] ?? '');

    if (empty($public_ipv6)) {
        return ['status' => 'error', 'msg' => '缺少IPv6地址参数'];
    }

    $requestData = 'hostname=' . urlencode($params['domain']) . '&public_ipv6=' . urlencode($public_ipv6);

    $data = [
        'url'  => '/api/ipv6/delete',
        'type' => 'application/x-www-form-urlencoded',
        'data' => $requestData,
    ];

    $res = lxdserver_Curl($params, $data, 'POST');

    if (isset($res['code']) && $res['code'] == 200) {
        return ['status' => 'success', 'msg' => $res['msg'] ?? 'IPv6绑定删除成功'];
    } else {
        return ['status' => 'error', 'msg' => $res['msg'] ?? 'IPv6绑定删除失败'];
    }
}

function lxdserver_ipv6list($params)
{
    
    $data = [
        'url'  => '/api/ipv6/list?hostname=' . urlencode($params['domain']),
        'type' => 'application/x-www-form-urlencoded', 
        'data' => [],
    ];

    $res = lxdserver_Curl($params, $data, 'GET');

    if (isset($res['code']) && $res['code'] == 200) {
        return ['status' => 'success', 'data' => $res['data'] ?? [], 'msg' => $res['msg'] ?? ''];
    } else {
        return ['status' => 'error', 'data' => [], 'msg' => $res['msg'] ?? '获取IPv6绑定列表失败'];
    }
}

function lxdserver_proxyadd($params)
{
    $proxy_enabled = ($params['configoptions']['proxy_enabled'] ?? 'false') === 'true';
    if (!$proxy_enabled) {
        return ['status' => 'error', 'msg' => 'Nginx反向代理功能已禁用，请联系管理员启用此功能。'];
    }
    
    parse_str(file_get_contents("php://input"), $post);

    $domain = trim($post['domain'] ?? '');
    $container_port = intval($post['container_port'] ?? 80);
    $description = trim($post['description'] ?? '');
    $ssl_enabled = ($post['ssl_enabled'] ?? 'false') === 'true';
    $ssl_type = trim($post['ssl_type'] ?? 'self-signed');
    $ssl_cert = trim($post['ssl_cert'] ?? '');
    $ssl_key = trim($post['ssl_key'] ?? '');

    if (empty($domain)) {
        return ['status' => 'error', 'msg' => '请输入域名'];
    }

    if (!preg_match('/^([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$/', $domain)) {
        return ['status' => 'error', 'msg' => '域名格式无效'];
    }

    $proxy_limit = intval($params['configoptions']['proxy_limit'] ?? 1);
    $current_count = lxdserver_getProxyCount($params);
    
    if ($current_count >= $proxy_limit) {
        return ['status' => 'error', 'msg' => "已达到反向代理数量上限（{$proxy_limit}个），无法继续添加"];
    }

    if ($ssl_enabled && $ssl_type === 'custom' && (empty($ssl_cert) || empty($ssl_key))) {
        return ['status' => 'error', 'msg' => '启用自定义SSL证书时，必须提供证书和私钥内容'];
    }

    $requestData = 'hostname=' . urlencode($params['domain']) . 
                   '&domain=' . urlencode($domain) . 
                   '&container_port=' . $container_port .
                   '&description=' . urlencode($description) .
                   '&ssl_enabled=' . ($ssl_enabled ? 'true' : 'false') .
                   '&ssl_type=' . urlencode($ssl_type);
    
    if ($ssl_enabled && $ssl_type === 'custom') {
        $requestData .= '&ssl_cert=' . urlencode($ssl_cert) .
                       '&ssl_key=' . urlencode($ssl_key);
    }

    $data = [
        'url'  => '/api/proxy/add',
        'type' => 'application/x-www-form-urlencoded',
        'data' => $requestData,
    ];

    $res = lxdserver_Curl($params, $data);

    if (isset($res['code']) && $res['code'] == 200) {
        return ['status' => 'success', 'msg' => '反向代理添加成功'];
    } else {
        return ['status' => 'error', 'msg' => $res['msg'] ?? '添加反向代理失败'];
    }
}

function lxdserver_proxydel($params)
{
    parse_str(file_get_contents("php://input"), $post);

    $domain = trim($post['domain'] ?? '');

    if (empty($domain)) {
        return ['status' => 'error', 'msg' => '缺少域名参数'];
    }

    $requestData = 'hostname=' . urlencode($params['domain']) . '&domain=' . urlencode($domain);

    $data = [
        'url'  => '/api/proxy/delete',
        'type' => 'application/x-www-form-urlencoded',
        'data' => $requestData,
    ];

    $res = lxdserver_Curl($params, $data);

    if (isset($res['code']) && $res['code'] == 200) {
        return ['status' => 'success', 'msg' => '反向代理删除成功'];
    } else {
        return ['status' => 'error', 'msg' => $res['msg'] ?? '删除反向代理失败'];
    }
}

function lxdserver_proxylist($params)
{
    $data = [
        'url'  => '/api/proxy/list?hostname=' . urlencode($params['domain']),
        'type' => 'application/x-www-form-urlencoded',
        'data' => [],
    ];

    $res = lxdserver_Curl($params, $data, 'GET');

    if (isset($res['code']) && $res['code'] == 200) {
        return ['status' => 'success', 'data' => $res['data'] ?? [], 'msg' => $res['msg'] ?? ''];
    } else {
        return ['status' => 'error', 'data' => [], 'msg' => $res['msg'] ?? '获取反向代理列表失败'];
    }
}

function lxdserver_proxycheck($params)
{
    parse_str(file_get_contents("php://input"), $post);
    
    $domain = trim($post['domain'] ?? '');
    
    if (empty($domain)) {
        return ['status' => 'error', 'msg' => '请输入域名'];
    }
    
    $data = [
        'url'  => '/api/proxy/check?domain=' . urlencode($domain),
        'type' => 'application/x-www-form-urlencoded',
        'data' => [],
    ];
    
    $res = lxdserver_Curl($params, $data, 'GET');
    
    if (isset($res['code']) && $res['code'] == 200) {
        return ['status' => 'success', 'data' => $res['data'] ?? [], 'msg' => $res['msg'] ?? ''];
    } else {
        return ['status' => 'error', 'msg' => $res['msg'] ?? '检查域名失败'];
    }
}

function lxdserver_getProxyCount($params)
{
    $data = [
        'url'  => '/api/proxy/list?hostname=' . urlencode($params['domain']),
        'type' => 'application/x-www-form-urlencoded',
        'data' => [],
    ];

    $res = lxdserver_Curl($params, $data, 'GET');

    if (isset($res['code']) && $res['code'] == 200 && is_array($res['data'])) {
        return count($res['data']);
    }

    return 0;
}