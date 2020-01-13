import json
import sys


VALIDATED_COMMANDS = [
    'remove',
    'priority',
    'static-file-size',
    'performance'
]

KEY_RESOURCE_TYPE = 'resource_type'
KEY_URL = 'url'
KEY_SIZE = 'size'
KEY_MIME_TYPE = 'mime_type'


def dump_object_to_json_file(obj, file_path):
    with open(file_path, 'w') as target_file:
        target_file.write(json.dumps(obj, indent=2))


def parse_har_file(har_file_path):
    with open(har_file_path, 'r') as har_file:
        har_file_json = json.load(har_file)
    return har_file_json


def save_har_file(har_file_object, har_file_path):
    with open(har_file_path, 'w') as target_file:
        target_file.write(json.dumps(har_file_object, indent=2))


def delete_content_field(har_file_json):
    entries = har_file_json['log']['entries']
    for entry in entries:
        if 'text' in entry['response']['content']:
            del entry['response']['content']['text']
    return har_file_json


def command_remove(argv):
    har_file_path = argv[1]
    output_file_name = argv[3]
    har_file_json = parse_har_file(har_file_path)
    content_field_removed = delete_content_field(har_file_json)
    save_har_file(content_field_removed, output_file_name)


def get_call_stack(item):
    """
    深度优先搜索，搜索到的所有依赖项都会被添加到 set 类型实例 deps 中。每一个 item
    对象均包含 callFrames 数组和 parent 对象。如果 parent 对象非空，则继续在
    parent 对象中递归寻找依赖项。如果不包含 parent 对象，则只需查找 callFrames
    对象中的内容。返回值是依赖项列表
    """
    result = []
    # 查找 callFrames 中包含的依赖项
    call_frames = item['callFrames']
    deps_from_call_frames = []
    for frame in call_frames:
        deps_from_call_frames.append(frame['url'])
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


def generate_priority_tree(har_file_json):
    entries = har_file_json['log']['entries']

    result = []
    root_request_url = ''

    for entry in entries:
        extracted_entry_info = {}

        request = entry['request']
        # method = request['method']
        url = request['url']

        response = entry['response']
        status = response['status']
        size = response['content']['size']
        # mime_type = response['content']['mimeType']
        priority = entry['_priority']
        resource_type = entry['_resourceType']

        if root_request_url == '':
            root_request_url = url
        initiator = entry['_initiator']

        extracted_entry_info['url'] = url
        extracted_entry_info['size'] = size
        extracted_entry_info['priority'] = priority
        extracted_entry_info['resource_type'] = resource_type

        if status != 404 and url.find('https://www.stormlin.com') >= 0:
            has_stack = 'stack' in initiator
            initiator_type = initiator['type']
            extracted_entry_info['initiator_type'] = initiator_type
            if initiator_type == 'parser' or initiator_type == 'other':
                extracted_entry_info['deps'] = [root_request_url]
                result.append(extracted_entry_info)
                continue
            if has_stack:
                # find all deps by deep first search
                deps_list = get_call_stack(initiator['stack'])
                # 从函数调用栈中提取文件调用顺序
                extracted_entry_info['deps'] = deps_list
                result.append(extracted_entry_info)

    return result


def sort_leaf_nodes_by_type_and_size(leaf_nodes):
    """
    对关键路径中的叶子节点按照资源类型和大小进行排序

    Args:
      leaf_nodes: 叶子节点列表

    Returns:
      排好序的叶子节点列表
    """
    document_list = []
    stylesheet_list = []
    script_list = []

    for resource in leaf_nodes:
        resource_type = resource['resource_type']
        if resource_type == 'document':
            document_list.append(resource)
            continue
        if resource_type == 'stylesheet':
            stylesheet_list.append(resource)
            continue
        if resource_type == 'script':
            script_list.append(resource)
            continue

    document_list.sort(key=lambda element: element['size'])
    stylesheet_list.sort(key=lambda element: element['size'])
    script_list.sort(key=lambda element: element['size'])

    result = []
    result.extend(document_list)
    result.extend(stylesheet_list)
    result.extend(script_list)

    return result


def command_priority(argv):
    """
    目前采用依赖图的入度区分不同类型的资源
    """
    har_file_path = argv[1]
    har_file_json = parse_har_file(har_file_path)
    priority_info_list = generate_priority_tree(har_file_json)

    # 筛选出关键路径中的资源
    critical_path = []
    for resource in priority_info_list:
        resource_type = resource['resource_type']
        if resource_type in ['document', 'script', 'stylesheet']:
            critical_path.append(resource)
            continue

    # 计算每一个文件的网络调用顺序
    for resource in critical_path:
        file_call_sequence = get_file_call_sequence(resource['deps'])
        resource['file_call_sequence'] = file_call_sequence

    # 计算文件在依赖图中的入度，入度越大越有可能影响后续渲染过程
    indegree_dict = {}
    for resource in critical_path:
        # 初始化
        indegree_dict[resource['url']] = 0

    for resource in critical_path:
        file_call_sequence = resource['file_call_sequence']
        for call in file_call_sequence:
            if call not in indegree_dict:
                indegree_dict[call] = 0
            else:
                indegree_dict[call] = indegree_dict[call] + 1

    # 添加到数据源中
    for resource in critical_path:
        url = resource['url']
        resource['indegree'] = indegree_dict[url]

    # 区分叶子节点和非叶子节点
    indegree_intermediate_nodes = []
    indegree_leaf_nodes = []
    for resource in critical_path:
        if resource['indegree'] == 0:
            # 叶子节点的入度为 0
            indegree_leaf_nodes.append(resource)
            continue
        # 中间节点的入度不为 0
        indegree_intermediate_nodes.append(resource)

    indegree_intermediate_nodes.sort(
        key=lambda element: element['indegree'], reverse=True)
    indegree_leaf_nodes = sort_leaf_nodes_by_type_and_size(indegree_leaf_nodes)

    # 计算文件在加载过程中的平均位置，越小越靠前，应当优先传输
    load_order = {}
    for resource in critical_path:
        # 初始化相关结构体
        url = resource['url']
        load_order[url] = {
            'order': 0,
            'count': 0
        }

    for resource in critical_path:
        url = resource['url']
        index = 1
        for call in resource['file_call_sequence']:
            # 计算该文件的相对加载顺序
            load_order[call]['order'] += index
            load_order[call]['count'] += 1
            index += 1

    for resource in critical_path:
        url = resource['url']
        resource['order'] = load_order[url]['order']
        resource['count'] = load_order[url]['count']

    # 区分叶子节点和非叶子节点
    load_order_intermediate_nodes = []
    load_order_leaf_nodes = []
    for resource in critical_path:
        url = resource['url']
        count = load_order[url]['count']
        if count == 0:
            # 叶子节点不会出现在其他资源的调用栈中
            load_order_leaf_nodes.append(resource)
            continue
        load_order_intermediate_nodes.append(resource)

    load_order_intermediate_nodes.sort(
        key=lambda element: element['order'] / element['count'])
    # for resource in load_order_intermediate_nodes:
    #     url = resource['url']
    #     order = load_order[url]['order']
    #     count = load_order[url]['count']
    #     print('  average_order = <%.1f>, url = <%s>' % (order / count, url))
    # for resource in load_order_leaf_nodes:
    #     print('  leaf node, size = <%.1f>KB, url = <%s>' % (resource['size'] / 1000.0, resource['url']))

    # 找出所有不在关键路径上的资源
    url_in_criticalp_path = set()
    for resource in critical_path:
        url_in_criticalp_path.add(resource['url'])
    resource_not_in_critical_path = []
    for resource in priority_info_list:
        url = resource['url']
        if url not in url_in_criticalp_path:
            resource_not_in_critical_path.append(resource)

    # 导出的顺序列表分两部分：
    # 第一部分：关键路径上的资源，采用指定顺序的方式传输
    # 第二部分：不在关键路径上的资源，采用简单文件分类的形式进行传输
    managed_resources = []
    managed_resources.extend(indegree_intermediate_nodes)
    managed_resources.extend(indegree_leaf_nodes)
    # 去除重复的资源
    added_url = set()
    temp = []
    for resource in managed_resources:
        url = resource['url']
        if url not in added_url:
            added_url.add(url)
            temp.append(resource)
    managed_resources = temp
    urls_to_dump = []
    for resource in managed_resources:
        urls_to_dump.append(resource['url'])
    print(urls_to_dump)

    dump_object_to_json_file(urls_to_dump, 'result.json')


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
    har_file_json = parse_har_file(har_file_path)

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
    pass


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
    if command == 'remove':
        command_remove(argv[1:])
        return
    elif command == 'priority':
        command_priority(argv[1:])
        return
    elif command == 'static-file-size':
        command_static_file_size(argv[1:])


if __name__ == "__main__":
    main(sys.argv[1:])
