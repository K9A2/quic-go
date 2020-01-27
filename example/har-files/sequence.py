# -*- coding: UTF-8 -*-
import json
import sys


VALIDATED_COMMANDS = [
    'preprocess',
    'static-file-size',
    'performance',
    # 从网络加载顺序生成加载顺序
    'network'
]

KEY_RESOURCE_TYPE = 'resource_type'
KEY_URL = 'url'
KEY_SIZE = 'size'
KEY_MIME_TYPE = 'mime_type'

CRITICAL_RESOURCE_TYPES = [
    'text/html',
    'application/javascript',
    'text/css',
    'font/woff2'
]


def dump_object_to_json_file(obj, file_path):
    with open(file_path, 'w') as target_file:
        target_file.write(json.dumps(obj, indent=2))


def load_json_file(json_file_path):
    with open(json_file_path, 'r') as har_file:
        har_file_json = json.load(har_file)
    return har_file_json


def delete_content_field(har_file_json):
    """
    逐条删除 text 字段
    """
    entries = har_file_json['log']['entries']
    for entry in entries:
        if 'text' in entry['response']['content']:
            del entry['response']['content']['text']
    return har_file_json


def check_headers(entry):
    headers = entry['request']['headers']
    for header in headers:
        if header['name'] == 'Host':
            if header['value'] == 'www.stormlin.com':
                return True
    return False


def check_status(entry):
    if entry['response']['status'] == 200:
        return True
    return False


def preprocess(har_file_json):
    """
    只提取 host 为 www.stormlin.com 和响应状态码为 200 OK 的条目的部分字段
    """
    entries = har_file_json['log']['entries']
    result = []
    for entry in entries:
        if check_headers(entry) and check_status(entry):
            new_entry = {
                'url': entry['request']['url'],
                'method': entry['request']['method'],
                'mimeType': entry['response']['content']['mimeType'],
                'size': entry['response']['content']['size'],
                'initiator': entry['_initiator']
            }
            result.append(new_entry)
    return { 'log': result }


def command_preprocess(argv):
    # 输入文件名
    har_file_path = argv[0]
    # 输出文件名
    output_file_name = argv[1]
    # 解析文件
    har_file_json = load_json_file(har_file_path)
    print('%s loaded' % har_file_path)
    # 逐条删除 text 字段
    content_field_removed = delete_content_field(har_file_json)
    # 只提取 host 为 www.stormlin.com 和响应状态码为 200 OK 的条目
    after_preprocess = preprocess(content_field_removed)
    # 保存到指定的输出文件中
    dump_object_to_json_file(after_preprocess, output_file_name)
    print('saved as: %s' % output_file_name)


def get_call_stack(item):
    """
    深度优先搜索，搜索到的所有依赖项同一个列表中。依赖项可以重复。
    每一个 item 对象均包含 callFrames 数组和 parent 对象。如果 parent 对象非空，
    则继续在 parent 对象中递归寻找依赖项。如果不包含 parent 对象，则只需查找
    callFrames 对象中的内容。返回值是依赖项列表
    """
    result = []
    # 查找 callFrames 中包含的依赖项
    call_frames = item['callFrames']
    deps_from_call_frames = []
    for frame in call_frames:
        deps_from_call_frames.append(get_file_name(frame['url']))
    result.extend(deps_from_call_frames)

    if 'parent' in item:
        # 含有 parent 对象，需要在此对象中进行递归查找
        deps_from_parent = get_call_stack(item['parent'])
        result.extend(deps_from_parent)
    return result


def get_file_call_sequence(call_stack_list):
    """
    从调用栈列表中提取文件调用顺序

    Args:
      call_stack_list: 调用栈列表

    Returns:
      从调用栈最深到最浅的文件调用顺序列表
    """
    call_stack_list.reverse()
    file_call_sequence = []
    added_files = set()
    for file_url in call_stack_list:
        if file_url not in added_files:
            file_call_sequence.append(file_url)
            added_files.add(file_url)
    return file_call_sequence


def get_file_name(url):
    """
    本函数从 url 中提取对应的资源的文件名
    """
    url = url.replace('https://www.stormlin.com/', '')
    if url == '':
        url = 'index.html'
    return url


def extract_deps_from_log(har_file_json):
    entries = har_file_json['log']

    result = []
    root_request_url = ''

    entries_added = set()
    for entry in entries:
        url = entry['url']
        mime_type = entry['mimeType']

        # print(mime_type)
        if mime_type not in CRITICAL_RESOURCE_TYPES:
            # 不处理非关键路径上的资源
            continue

        if url in entries_added:
            # 不处理重复发送的请求
            continue
        entries_added.add(url)

        initiator = entry['initiator']
        size = entry['size']

        new_entry = {
            'url': get_file_name(url),
            'size': size,
            'mime_type': mime_type
        }

        if root_request_url == '':
            root_request_url = url

        initiator_type = initiator['type']
        if initiator_type == 'parser' or initiator_type == 'other':
            new_entry['dep'] = [get_file_name(root_request_url)]
            result.append(new_entry)
            continue

        has_stack = 'stack' in initiator
        if has_stack:
            # 用深度优先搜索找出所有依赖项，然后逆序并去重
            deps_list = get_call_stack(initiator['stack'])
            deps_list.reverse()
            added_deps = set()
            final_dep_list = []
            for dep in deps_list:
                if dep not in added_deps:
                    final_dep_list.append(dep)
                    added_deps.add(dep)
            new_entry['dep'] = final_dep_list
            # 从函数调用栈中提取文件调用顺序
            result.append(new_entry)

    return result


def sort_by_type_and_size(layer_nodes):
    """
    对关键路径中的叶子节点按照资源类型和大小进行排序

    Args:
      leaf_nodes: 叶子节点列表

    Returns:
      排好序的叶子节点列表
    """
    document_list = []
    stylesheet_list = []
    font_list = []
    script_list = []
    for node in layer_nodes:
        if node['mime_type'] == 'text/html':
            document_list.append(node)
        elif node['mime_type'] == 'text/css':
            stylesheet_list.append(node)
        elif node['mime_type'] == 'font/woff2':
            font_list.append(node)
        else:
            script_list.append(node)
    document_list.sort(key=lambda element: element['size'])
    stylesheet_list.sort(key=lambda element: element['size'])
    font_list.sort(key=lambda element: element['size'])
    script_list.sort(key=lambda element: element['size'])

    ordered_nodes = []
    ordered_nodes.extend(document_list)
    ordered_nodes.extend(stylesheet_list)
    ordered_nodes.extend(font_list)
    ordered_nodes.extend(script_list)

    return ordered_nodes


def unset(block_list):
    for block in block_list:
        block['is_set'] = False


def get_transmission_order(block_list):
    # 已经添加的 url 列表
    added_url = []
    related_to_index = False
    for block in block_list:
        dep_list = block['dep_list']
        for dep in dep_list:
            if dep == 'index.html':
                related_to_index = True
                continue
            if dep not in added_url:
                added_url.append(dep)
    if related_to_index:
        added_url.insert(0, 'index.html')
    return added_url


def merge(has_successor_info, no_successor_info):
    added_url = []
    for url in has_successor_info:
        if url not in added_url and url != 'index.html':
            added_url.append(url)
    for url in no_successor_info:
        if url not in added_url and url != 'index.html':
            added_url.append(url)
    return added_url


def get_final_order(block_list):
    order = []
    for block in block_list:
        order.append(block['url'])
    return order


def sort_by_indegree_and_file_type(log_list):
    file_control_block_dict = {}
    # 创建并初始化控制字典
    for log in log_list:
        file_control_block_dict[log['url']] = {
            'url': log['url'],
            'mime_type': log['mime_type'],
            'size': log['size'],
            # 入度
            'indegree': 0,
            # 在依赖数中的第几层
            'layer': -1,
            # 加载依赖项列表
            'dep_list': [],
            # 是否已确定顺序
            'is_set': False
        }

    # 逐个 log 条目检查其 deps 列表并计算对应节点的入度
    for log in log_list:
        dep_list = log['dep']
        file_control_block_dict[log['url']]['dep_list'] = log['dep']
        for dep in dep_list:
            file_control_block_dict[dep]['indegree'] += 1

    # 层序排序
    result_dict = {}
    layer_list = [[]]
    # 最大层数
    max_layer_index = -1

    # 把 index.html 设为 0 层，其余请求在 1-n 层
    file_control_block_dict['index.html']['layer'] = 0
    file_control_block_dict['index.html']['is_set'] = True

    result_dict[0] = [file_control_block_dict['index.html']]
    layer_list[0] = [file_control_block_dict['index.html']]

    # 分层
    for key, value in file_control_block_dict.items():
        # 跳过已确定顺序的控制块
        if value['is_set'] == False:
            # 确定是否所有依赖项都已经被确定顺序，然后确定所有依赖项中最深的在第 n 层，
            # 并把本控制块放在 n+1 层
            deepest_layer = 0
            # 总依赖项数目
            all_dep_count = len(value['dep_list'])
            # 已经被确定了顺序的依赖项个数
            set_dep_count = 0
            for dep in value['dep_list']:
                if file_control_block_dict[dep]['is_set'] == False:
                    # 仍有依赖项还没有确定顺序
                    break
                # 更新最深层数值
                if file_control_block_dict[dep]['layer'] > deepest_layer:
                    deepest_layer = file_control_block_dict[dep]['layer']
                # 新增一个确定了顺序的依赖项
                set_dep_count += 1
            if all_dep_count == set_dep_count:
                # 所有依赖项都已经设定了顺序，则可以把本控制块放到 n+ 1 层
                target_layer = deepest_layer + 1
                if target_layer not in result_dict:
                    # 首次添加该层
                    result_dict[target_layer] = []
                if target_layer > max_layer_index:
                    # 更新最大层数
                    max_layer_index = target_layer
                file_control_block_dict[key]['is_set'] = True
                file_control_block_dict[key]['layer'] = target_layer
                result_dict[target_layer].append(value)

    # 逐层确定那些是有后继节点，那些是无后继节点
    layer_control_dict = {}
    layer_index = 0
    for layer_index, layer_nodes in result_dict.items():
        # 有后继节点列表
        has_successor_node_list = []
        # 无后继节点列表
        no_successor_node_list = []
        for node in layer_nodes:
            if node['indegree'] > 0:
                has_successor_node_list.append(node)
            else:
                no_successor_node_list.append(node)
        unset(has_successor_node_list)
        unset(no_successor_node_list)
        layer_control_dict[layer_index] = {
            # 有后继的借点尚未有序排列
            'has_successor': has_successor_node_list,
            # 无后继的节点已有序排列
            'no_successor': sort_by_type_and_size(no_successor_node_list)
        }

    # 首先设定无后继的节点的顺序
    all_layer_count = len(layer_control_dict.keys())
    deepest_layer = all_layer_count - 1
    # 需要处理的下一层
    next_layer = deepest_layer

    # 从有后继的几点中提取传输顺序
    order_from_has_successor = get_transmission_order(layer_control_dict[next_layer]['has_successor'])
    # 从无后继的节点中提取传输顺序
    order_from_no_successor = get_transmission_order(layer_control_dict[next_layer]['no_successor'])
    # 合并本层获得的顺序信息
    order_of_this_layer = merge(order_from_has_successor, order_from_no_successor)
    # 逆序确定最佳传输顺序
    while next_layer > 1:
        # 转移至上一层
        next_layer -= 1
        # 根据从下一层获得的顺序信息设定本层 has_successor 节点的顺序
        new_has_successor = []
        for url in order_of_this_layer:
            for block in layer_control_dict[next_layer]['has_successor']:
                if url in block['url']:
                    new_has_successor.append(block)
        layer_control_dict[next_layer]['has_successor'] = new_has_successor

        # 写入本层的传输顺序
        order_from_has_successor = get_transmission_order(layer_control_dict[next_layer]['has_successor'])
        order_from_no_successor = get_transmission_order(layer_control_dict[next_layer]['no_successor'])
        # 合并本层获得的顺序信息以供上层使用
        order_of_this_layer = merge(order_from_has_successor, order_from_no_successor)

    # 提取最终的传输顺序
    current_layer = 0
    final_order = []
    while current_layer < len(layer_control_dict.keys()):
        final_order.extend(get_final_order(layer_control_dict[current_layer]['has_successor']))
        final_order.extend(get_final_order(layer_control_dict[current_layer]['no_successor']))
        current_layer += 1

    return final_order


def command_network(argv):
    """
    目前采用依赖图的入度区分不同类型的资源
    """
    har_file_path = argv[0]
    output_file_name = argv[1]
    har_file_json = load_json_file(har_file_path)
    nodes_with_deps = extract_deps_from_log(har_file_json)
    final_order = sort_by_indegree_and_file_type(nodes_with_deps)
    dump_object_to_json_file({ 'final_order': final_order }, output_file_name)


def remove_deulicated_resource(resource_list):
    """
    移除所有具有重复 url 的资源，并返回移除重复项之后的资源列表
    """
    url_added = set()
    unduplicated_resources = []
    for resource in resource_list:
        if resource[KEY_URL] not in url_added:
            url_added.add(resource[KEY_URL])
            unduplicated_resources.append(resource)
    return unduplicated_resources


def command_static_file_size(argv):
    """
    根据文件类型和大小生成传输顺序
    """
    har_file_path = argv[1]
    output_file_path = argv[3]
    har_file_json = load_json_file(har_file_path)

    # 把所有资源分类为以下资源
    highest_priority_list = []
    highest_priority_resource_type = ['document']

    high_priority_list = []
    high_priority_resource_type = ['stylesheet']

    normal_priority_list = []
    normal_priority_resource_type = ['script']

    low_priority_list = []
    low_priority_resource_type = ['font']

    lowest_priority_list = []
    lowest_priority_resource_type = ['image']

    background_priority_list = []
    background_priority_resource_type = ['xhr', 'manifest', 'other']

    # 从 har 文件中读取所有请求，按照文件类型分类之后再按照大小排序
    entries = har_file_json['log']['entries']
    print('count: %s' % len(entries))
    for resource in entries:
        url = resource['request']['url']
        mime_type = resource['response']['content']['mimeType']
        size = resource['response']['content']['size']
        resource_type = resource['_resourceType']

        extracted_resource = {
            KEY_URL: url,
            KEY_MIME_TYPE: mime_type,
            KEY_SIZE: size,
            KEY_RESOURCE_TYPE: resource_type
        }

        if resource_type in highest_priority_resource_type:
            highest_priority_list.append(extracted_resource)
        elif resource_type in high_priority_resource_type:
            high_priority_list.append(extracted_resource)
        elif resource_type in normal_priority_resource_type:
            normal_priority_list.append(extracted_resource)
        elif resource_type in low_priority_resource_type:
            low_priority_list.append(extracted_resource)
        elif resource_type in lowest_priority_resource_type:
            lowest_priority_list.append(extracted_resource)
        elif resource_type in background_priority_resource_type:
            background_priority_resource_type.append(extracted_resource)
        else:
            # 找不到合适的队列插入此资源
            print('error: no corresponding queue available, resource_type = <%s>, \
                  url = <%s>' % (resource_type, url))

    # 对分类的各种资源按照大小进行排序
    highest_priority_list.sort(key=lambda element: element[KEY_SIZE])
    highest_priority_list = remove_deulicated_resource(highest_priority_list)

    high_priority_list.sort(key=lambda element: element[KEY_SIZE])
    high_priority_list = remove_deulicated_resource(high_priority_list)

    normal_priority_list.sort(key=lambda element: element[KEY_SIZE])
    normal_priority_list = remove_deulicated_resource(normal_priority_list)

    low_priority_list.sort(key=lambda element: element[KEY_SIZE])
    low_priority_list = remove_deulicated_resource(low_priority_list)

    lowest_priority_list.sort(key=lambda element: element[KEY_SIZE])
    lowest_priority_list = remove_deulicated_resource(lowest_priority_list)

    background_priority_list.sort(key=lambda element: element[KEY_SIZE])
    background_priority_list = remove_deulicated_resource(background_priority_list)

    sequence = {
        'highest': [],
        'high': [],
        'normal': [],
        'low': [],
        'lowest': [],
        'background': []
    }

    # 结果只保留资源的 url
    for resource in highest_priority_list:
        sequence['highest'].append(resource[KEY_URL])
    for resource in high_priority_list:
        sequence['high'].append(resource[KEY_URL])
    for resource in normal_priority_list:
        sequence['normal'].append(resource[KEY_URL])
    for resource in low_priority_list:
        sequence['low'].append(resource[KEY_URL])
    for resource in lowest_priority_list:
        sequence['lowest'].append(resource[KEY_URL])
    for resource in background_priority_list:
        sequence['backgroud'].append(resource[KEY_URL])

    dump_object_to_json_file(sequence, output_file_path)


def command_performance(argv):
    """
    根据 chrome performance 日志生成最优加载顺序
    """
    # 统计本文件中一共包含了哪几类数据，并按照分类输出到不同的文件中
    result = []
    critical_categories = [
        'blink,devtools.timeline.json',
        'blink,user_timing',
        'devtools.timeline,rail',
        'devtools.timeline',
        'loading,rail,devtools.timeline'
    ]
    with open('profile-sample.json', 'r') as json_file:
        data = json.load(json_file)
        for log in data:
            cat = log['cat']
            if cat in critical_categories:
                result.append(log)
    dump_object_to_json_file(result, 'filtered_result.json')


def main(argv):
    # parse arguments from command line
    if len(argv) <= 0:
        print('error: no command provided')
        return

    # extract command to be executed
    command = argv[0]
    if not (command in VALIDATED_COMMANDS):
        print('error: unsupported command <%s>' % command)
        return

    # execute command
    if command == 'preprocess':
        # 清空文件内的 text 字段
        print('executing command <%s>' % command)
        command_preprocess(argv[1:])
        return
    elif command == 'network':
        print('executing command <%s>' % command)
        command_network(argv[1:])
        return
    elif command == 'static-file-size':
        command_static_file_size(argv[1:])
    elif command == 'performance':
        command_performance(argv[1:])


if __name__ == "__main__":
    main(sys.argv[1:])
