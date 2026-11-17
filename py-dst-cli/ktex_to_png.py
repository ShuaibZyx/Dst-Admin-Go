# -*- coding: utf-8 -*-
"""KTEX(DXT5) -> PNG 图集解码器。Klei 的 .tex 封装 BC3(DXT5) 像素 + mipmap 链。
头部: "KTEX" + 标志位，mip0 宽高以大端 u16 存于偏移 88/90（parse_world_setting.py 同款读法）。
解码路径：把 mip0 裸数据套上 DDS 头，交给 Pillow 解 BC3。"""
import struct
import sys
from PIL import Image


def ktex_to_png(src, dst):
    data = open(src, 'rb').read()
    assert data[:4] == b'KTEX', 'not a KTEX file: %s' % src
    width = int.from_bytes(data[88:90], 'big')
    height = int.from_bytes(data[90:92], 'big')
    # mip0 = BC3 每像素 1 字节（按 4x4 块，最小 16 字节），从文件尾部链式尺寸反推偏移：
    # 头部到像素数据之间是各 mip 的偏移表；这里用总量差倒推 mip0 起点。
    mips = [(width, height)]
    w, h = width, height
    while w > 1 or h > 1:
        w, h = max(1, w // 2), max(1, h // 2)
        mips.append((w, h))
    pixel_bytes = sum(((mw + 3) // 4) * ((mh + 3) // 4) * 16 for mw, mh in mips)
    pixel_start = len(data) - pixel_bytes
    mip0_size = ((width + 3) // 4) * ((height + 3) // 4) * 16
    body = data[pixel_start:pixel_start + mip0_size]

    dds = struct.pack('<4sI', b'DDS ', 124)
    dds += struct.pack('<6I', 0x81007, height, width, mip0_size, 0, 0)  # flags/h/w/linearsize/depth/mips
    dds += b'\x00' * 44  # dwReserved1[11]
    dds += struct.pack('<II4s5I', 32, 0x4, b'DXT5', 0, 0, 0, 0, 0)  # ddspf: size/flags/fourcc/masks
    dds += b'\x00' * 20  # dwCaps1-4 + dwReserved2
    dds += body
    img = Image.open(__import__('io').BytesIO(dds)).convert('RGBA')
    assert img.size == (width, height), 'size mismatch %s != %s' % (img.size, (width, height))
    # tex 行序自底向上，UV 数学按 PNG 顶行取格，故整图垂直翻转（与既有图集管线一致）
    img = img.transpose(Image.FLIP_TOP_BOTTOM)
    # 面板精灵窗口为 64px 格（LevelViewer 的背景平绕公式以 64px 切格）。
    # 游戏 tex 图集格恒为 128px，按 1/2 缩放对齐既有图集约定（customization_shipwrecked.png 同款）。
    img = img.resize((width // 2, height // 2), Image.LANCZOS)
    img.save(dst)
    return width // 2, height // 2


if __name__ == '__main__':
    w, h = ktex_to_png(sys.argv[1], sys.argv[2])
    print('decoded %sx%s -> %s' % (w, h, sys.argv[2]))
