import json
import base64
import urllib.request

DOAMIN_INDEX = 2
RESOURCE_INDEX = -1


def write(file_path, content, mode):
    with open(file_path, mode) as output_file:
        output_file.write(content)


def is_text_file(mime_type):
    text_file_types = [
        'text/html', 'text/plain', 'text/javascript', 'text/css', 'application/json',
        'application/x-javascript', 'application/javascript'
    ]
    # binary_file_types = [
    #     'font/woff2', 'image/jpeg', 'image/webp', 'image/gif'
    # ]

    if any(mime_type in s for s in text_file_types):
        return True
    return False


def main():
    harfile_path = 'www.sina.com.cn.har'
    output_folder = './sina/'
    harfile = open(harfile_path)
    harfile_json = json.loads(harfile.read())

    entries = harfile_json['log']['entries']
    numEntries = len(entries)
    print('numEntries: %d' % numEntries)

    domains = []
    index = 1
    # 已经保存过的资源
    visited = set()
    for entry in entries:
        # parse fields from request
        request = entry['request']
        params = request['url'].split('/')

        url = request['url']
        print('processing: %s' % url)

        method = request['method']
        resource_name = params[RESOURCE_INDEX]
        domain = params[DOAMIN_INDEX]
        decoded = False

        # parse fields from response
        response = entry['response']
        mime_type = response['content']['mimeType']
        encoding = ''
        if 'encoding' in response['content'] or 'text' not in response['content']:
            # encoding = response['content']['encoding']
            # 对于已经编码的二进制文件，采用从源服务器下载的方式进行处理，而非从尝试从
            # text 字段中解码的方式，以避免在解码过程中出现错误
            output_filename = output_folder + resource_name
            # print('resource: %s' % (url))
            # print('  download as %s' % output_filename)
            urllib.request.urlretrieve(url, output_filename)
            continue
        # if encoding == 'base64':
        #     content = base64.b64decode(content)
        #     decoded = True

        content = response['content']['text']
        if resource_name == '':
            resource_name = 'index.html'
        print('extracted from har text: %s' % resource_name)
        if is_text_file(mime_type):
            if decoded:
                content = content.decode('utf-8')
            write(output_folder + resource_name, content, 'w')
        else:
            write(output_folder + resource_name, content, 'wb')


if __name__ == "__main__":
    main()
