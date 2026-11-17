# -*- coding: utf-8 -*-
"""世界设置目录构建器：以游戏机器真相重建 dst_world_setting.json + 模组注册表。

数据源（全部为游戏/模组的注册定义，权威优先级高于任何人工整理）：
  1. 游戏 scripts/map/customize.lua —— 原版 WORLDGEN/WORLDSETTINGS 选项（world 过滤、master_controlled、图标名、值域）
  2. IA(1467214795) + IA-Core(3435352667) 的 modservercreationmain.lua —— 海难/火山选项注册 + vanilla_options_in_islands/volcano 白名单
  3. 樱林(1289779251) 的 cherry_customizations.lua + init/init_worldgen.lua —— 樱林选项注册（默认 world={forest,shipwrecked,volcanoworld}）
  4. 名称：原版 chinese_s.po / IA ia_sc.po / 樱林 strings.lua(en)+ch/strings.lua(zh)
  5. 图标 UV：各图集 XML + KTEX 头部尺寸（与 parse_world_setting.py 同一套数学）

输出：
  dst-admin-web/public/misc/dst_world_setting.json     —— 静态目录（森林/洞穴=原版；海难/火山=原版白名单+IA）
  dst-admin-web/src/types/modWorldOptions.json         —— 运行时注册表（模组选项：文件里有才显示）
  stdout                                               —— 与截图预期的条目数对账
"""
import io
import json
import os
import re
import struct
import zipfile

ROOT = r'D:\New World\dst-admin-shuaib'
GAME_DATA = r'D:\DST-Server\dst-dedicated-server\data'
MODS = r'D:\DST-Server\mods\steamapps\workshop\content\322330'
CHERRY = r'C:\Users\Administrator\Documents\Klei\steamcmd\steamapps\workshop\content\322330\1289779251'
PUB_JSON = os.path.join(ROOT, r'dst-admin-web\public\misc\dst_world_setting.json')
REG_JSON = os.path.join(ROOT, r'dst-admin-web\src\types\modWorldOptions.json')

LOC = {'forest': 'forest', 'cave': 'cave', 'shipwrecked': 'shipwrecked', 'volcano': 'volcanoworld'}
# master_controlled 项（季节长度/中毒/起始季节等）不再按世界名固化：
# 目录全量收录并打 master 标记，运行时按 集群主世界(server_ini.is_master) 决定显隐——游戏同款语义
# （IsMasterLevel = location_index==1，即集群第一/主世界才显示）。

# ---------------------------------------------------------------- 值域（zh/en 标签）
DOMAIN_KEYS = {
    'freq5': [('SLIDENEVER', 'never'), ('SLIDERARE', 'rare'), ('SLIDEDEFAULT', 'default'), ('SLIDEOFTEN', 'often'), ('SLIDEALWAYS', 'always')],
    'wfreq8': [('SLIDENEVER', 'never'), ('SLIDERARE', 'rare'), ('SLIDEUNCOMMON', 'uncommon'), ('SLIDEDEFAULT', 'default'), ('SLIDEOFTEN', 'often'), ('SLIDEMOSTLY', 'mostly'), ('SLIDEALWAYS', 'always'), ('SLIDEINSANE', 'insane')],
    'speed6': [('SLIDENEVER', 'never'), ('SLIDEVERYSLOW', 'veryslow'), ('SLIDESLOW', 'slow'), ('SLIDEDEFAULT', 'default'), ('SLIDEFAST', 'fast'), ('SLIDEVERYFAST', 'veryfast')],
    'ed': [('DISABLED', 'none'), ('ENABLED', 'always')],
    'yesno': [('SLIDENEVER', 'never'), ('SLIDEDEFAULT', 'default')],
    'slen': [('SLIDENEVER', 'noseason'), ('SLIDEVERYSHORT', 'veryshortseason'), ('SLIDESHORT', 'shortseason'), ('SLIDEDEFAULT', 'default'), ('SLIDELONG', 'longseason'), ('SLIDEVERYLONG', 'verylongseason'), ('RANDOM', 'random')],
    'ee': [('SLIDEDEFAULT', 'default'), ('SLIDEALWAYS', 'enabled')],
    'swap': [('CLASSIC', 'classic'), ('SLIDEDEFAULT', 'default'), ('HIGHLYRANDOM', 'highly random')],
}
DESCVAR2DOMAIN = {
    'frequency_descriptions': 'freq5', 'worldgen_frequency_descriptions': 'wfreq8',
    'speed_descriptions': 'speed6', 'enableddisabled_descriptions': 'ed',
    'yesno_descriptions': 'yesno', 'season_length_descriptions': 'slen',
    'extraevent_descriptions': 'ee', 'starting_swaps_descriptions': 'swap',
    'ocean_worldgen_frequency_descriptions': 'wfreq8', 'ocean_yesno_descriptions': 'yesno',
    'petrification_descriptions': 'freq5', 'disease_descriptions': 'freq5',
}
# 分类默认值域（item 未声明 desc 时继承）
CAT_DOMAIN = {
    'WORLDGEN': {'resources': 'wfreq8', 'animals': 'wfreq8', 'monsters': 'wfreq8', 'misc': 'yesno', 'global': 'yesno'},
    'WORLDSETTINGS': {'resources': 'speed6', 'animals': 'freq5', 'monsters': 'freq5', 'giants': 'freq5',
                      'events': 'ee', 'misc': 'freq5', 'global': 'yesno', 'survivors': 'ed',
                      'portal_resources': 'freq5', 'lunar_mutations': 'yesno', 'tropical_seasons': 'slen'},
}
# 固定值域（注册里是字面量表）
SW_SEASON_START = ('ssw', [('SHIPWRECKED_DEFAULT', 'default'), ('HURRICANE', 'wet'), ('MONSOON', 'green'),
                           ('DRY', 'dry'), ('MILD_MONSOON', 'mild|green'), ('HURRICANE_DRY', 'wet|dry'),
                           ('RANDOM', 'mild|wet|green|dry')])

# 各世界/页签的分类（顺序与文案=用户截图）
# 分类骨架四世界统一（文案/顺序）；各世界内容由 world 过滤 + master 标记塑造，
# master 项渲染端按集群主世界(is_master)显隐——与游戏 location_index==1 同语义
CATS_WGS = [('global', '全局', 'Global', 0), ('events', '活动', 'Events', 1), ('survivors', '冒险家', 'Survivors', 2),
            ('misc', '世界', 'World', 3), ('resources', '资源再生', 'Resource Regrowth', 4),
            ('portal_resources', '非自然传送门资源', 'Unnatural Portal Resources', 5), ('animals', '生物', 'Creatures', 6),
            ('monsters', '敌对生物', 'Hostile Creatures', 7), ('giants', '巨兽', 'Giants', 8),
            ('lunar_mutations', '月亮变异', 'Lunar Mutations', 9)]
CATS_WGN = [('global', '全局', 'Global', 0), ('misc', '世界', 'World', 1), ('resources', '资源', 'Resources', 2),
            ('animals', '生物以及刷新点', 'Creatures and Spawners', 3),
            ('monsters', '敌对生物以及刷新点', 'Hostile Creatures and Spawners', 4)]
STRUCT = {w: {'WORLDSETTINGS': CATS_WGS, 'WORLDGEN': CATS_WGN}
          for w in ('forest', 'cave', 'shipwrecked', 'volcano')}

# 模组图集 → 面板图片 URL（static 目录里已转好）
ATLAS_URL = {
    'worldgen_customization': './misc/worldgen_customization_game.webp',
    'worldsettings_customization': './misc/worldsettings_customization_game.webp',
    'customization_shipwrecked': '/misc/customization_shipwrecked.png',
    'customization_core': '/misc/customization_core.png',
    'worldgen_cherry': '/misc/worldgen_cherry.png',
}
ATLAS_FILE = {
    'worldgen_customization': (GAME_DATA, 'images/worldgen_customization.xml'),
    'worldsettings_customization': (GAME_DATA, 'images/worldsettings_customization.xml'),
    'customization_shipwrecked': (os.path.join(MODS, '1467214795'), 'images/hud/customization_shipwrecked.xml'),
    'customization_core': (os.path.join(MODS, '3435352667'), 'images/hud/customization_core.xml'),
    'worldgen_cherry': (CHERRY, 'images/worldgen_cherry.xml'),
}


# ---------------------------------------------------------------- 图集 UV
def atlas_meta(name):
    """解析图集 XML + KTEX 尺寸，返回 {width,height,item_size,img_items}。与 parse_world_setting.py 同数学。"""
    base, rel = ATLAS_FILE[name]
    xml = open(os.path.join(base, rel), encoding='utf-8').read()
    texfile = re.search(r'filename="([^"]+)"', xml).group(1)
    texpath = os.path.join(base, os.path.dirname(rel), texfile)
    head = open(texpath, 'rb').read(96)
    width, height = int.from_bytes(head[88:90], 'big'), int.from_bytes(head[90:92], 'big')
    u1, u2 = re.search(r'u1="([^"]+?)"\s*?u2="([^"]+?)"', xml).groups()
    item_size_px = int(width / round(1 / (float(u2) - float(u1))))
    num_w, num_h = width / item_size_px, height / item_size_px
    items = {m[0]: {'x': round(float(m[1]) * num_w) / num_w, 'y': 1 - round(float(m[2]) * num_h) / num_h}
             for m in re.findall(r'<Element\s+name="([^"]+?)"\s*u1="([^"]+?)"[\d\D]*?v2="([^"]+?)"', xml)}
    return {'name': name, 'width': width, 'height': height, 'item_size': item_size_px, 'items': items}


# ---------------------------------------------------------------- 语言名
def load_po_names():
    """原版 po：STRINGS.UI.CUSTOMIZATIONSCREEN.* 与 SANDBOXMENU.* 的 zh/en。"""
    z = zipfile.ZipFile(os.path.join(GAME_DATA, r'databundles\scripts.zip'))
    data = z.read('scripts/languages/chinese_s.po').decode('utf-8', 'replace')
    zh, en = {}, {}
    for m in re.finditer(r'msgctxt "([^"]+)"\r?\nmsgid "([^"]*)"\r?\nmsgstr "([^"]*)"', data):
        zh[m.group(1)] = m.group(3)
        en[m.group(1)] = m.group(2)
    return zh, en


def load_ia_names():
    data = open(os.path.join(MODS, '3435352667', 'languages', 'ia_sc.po'), encoding='utf-8', errors='replace').read()
    zh, en = {}, {}
    for m in re.finditer(r'msgctxt "STRINGS\.UI\.CUSTOMIZATIONSCREEN\.([^"]+)"\r?\nmsgid "([^"]*)"\r?\nmsgstr "([^"]*)"', data):
        en[m.group(1)] = m.group(2)
        zh[m.group(1)] = m.group(3)
    return zh, en


def load_cherry_names():
    def grab(path):
        src = open(path, encoding='utf-8', errors='replace').read()
        return {m.group(1): m.group(2) for m in
                re.finditer(r'STRINGS\.UI\.CUSTOMIZATIONSCREEN\.([A-Z0-9_]+)\s*=\s*"([^"]+)"', src)}
    return grab(os.path.join(CHERRY, 'scripts', 'cherry_strings', 'ch', 'strings.lua')), \
           grab(os.path.join(CHERRY, 'scripts', 'cherry_strings', 'strings.lua'))


# ---------------------------------------------------------------- 原版 OPTIONS
def _strip(src):
    """去掉 -- 行注释（这些注册文件的字符串字面量里不含 --）。"""
    return re.sub(r'--[^\n]*', '', src)


def _balanced(seg):
    """从 seg[0]=='{' 处配平取函数体，返回 (body, end_index)。"""
    depth, i = 0, 0
    while i < len(seg):
        c = seg[i]
        if c == '{':
            depth += 1
        elif c == '}':
            depth -= 1
            if depth == 0:
                return seg[1:i], i + 1
        i += 1
    return seg[1:], i


def _entries(body, key_pat):
    """遍历 body 里 key_pat 匹配的 "key" = { ... }（配平体），产出 (key, inner)。
    key_pat 不得消耗左花括号——配平从 '{' 开始。"""
    pos = 0
    while True:
        m = re.search(key_pat, body[pos:])
        if not m:
            return
        key = m.group(1)
        rest = body[pos + m.end():]  # 以 '{' 开头
        inner, consumed = _balanced(rest)
        yield key, inner
        pos = pos + m.end() + consumed


def parse_customize():
    """regex 解析 customize.lua 的 WORLDGEN_GROUP / WORLDSETTINGS_GROUP。
    返回 {TAB: {cat: {'order','desc_var','atlas','items': {key: {...}}}}}"""
    z = zipfile.ZipFile(os.path.join(GAME_DATA, r'databundles\scripts.zip'))
    src = _strip(z.read('scripts/map/customize.lua').decode('utf-8', 'replace'))
    out = {}
    for tab, marker in [('WORLDGEN', 'local WORLDGEN_GROUP = {'), ('WORLDSETTINGS', 'local WORLDSETTINGS_GROUP = {')]:
        start = src.index(marker) + len('local WORLDGEN_GROUP = {') - 1
        block, _ = _balanced(src[start:])
        cats = {}
        for cname, body in _entries(block, r'\["([a-z_]+)"\]\s*=\s*(?=\{)'):
            order = re.search(r'\border\s*=\s*(\d+)', body)
            desc_var = re.search(r'\bdesc\s*=\s*([a-z_0-9]+)', body)
            atlas = re.search(r'atlas\s*=\s*"images/([a-z_]+)\.xml"', body)
            items = {}
            ipos = body.find('items')
            iseg = body[ipos:] if ipos >= 0 else ''
            for key, ib in _entries(iseg, r'\["([a-z0-9_]+)"\]\s*=\s*(?=\{)'):
                val = re.search(r'value\s*=\s*"([^"]*)"', ib)
                img = re.search(r'image\s*=\s*"([^"]*)"', ib)
                world = re.search(r'world\s*=\s*(?=\{)([^}]*)\}', ib)
                iorder = re.search(r'\border\s*=\s*(\d+)', ib)
                desc_v = re.search(r'\bdesc\s*=\s*([a-z_0-9]+)', ib)
                items[key] = {
                    'value': val.group(1) if val else 'default',
                    'image': img.group(1) if img else None,
                    'world': re.findall(r'"([^"]+)"', world.group(1)) if world else None,
                    'order': int(iorder.group(1)) if iorder else None,
                    'master': 'master_controlled' in ib,
                    'desc_var': desc_v.group(1) if desc_v else None,
                }
            cats[cname] = {'order': int(order.group(1)) if order else 0,
                           'desc_var': desc_var.group(1) if desc_var else None,
                           'atlas': atlas.group(1) if atlas else 'worldgen_customization',
                           'items': items}
        out[tab] = cats
    return out


def parse_vanilla_whitelists():
    """IA 把这些原版项扩展进岛屿/火山世界。"""
    src = open(os.path.join(MODS, '1467214795', 'modservercreationmain.lua'), encoding='utf-8', errors='replace').read()
    def grab(name):
        m = re.search(r'local %s = \{(.*?)\n\}' % name, src, re.S)
        return re.findall(r'"([a-z0-9_]+)"', m.group(1))
    return grab('vanilla_options_in_islands'), grab('vanilla_options_in_volcano')


# ---------------------------------------------------------------- IA/樱林注册表
def parse_reg_table(path, table_name):
    """解析 modservercreationmain.lua 的 {group: {key = {value=..., image=..., desc=..., world=..., order=..., master_controlled=...}}}"""
    src = _strip(open(path, encoding='utf-8', errors='replace').read())
    m = re.search(r'local %s = \{' % table_name, src)
    if not m:
        return {}
    block, _ = _balanced(src[m.end() - 1:])  # 模式含 '{'，回退到括号处配平
    out = {}
    for gname, body in _entries(block, r'\b([a-z_]+)\s*=\s*(?=\{)'):
        items = {}
        for key, ib in _entries(body, r'\b([a-z0-9_]+)\s*=\s*(?=\{)'):
            val = re.search(r'value\s*=\s*"([^"]*)"', ib)
            img = re.search(r'image\s*=\s*"([^"]*)"', ib)
            world = re.search(r'world\s*=\s*(\{[^}]*\}|[a-z_]+)', ib)
            iorder = re.search(r'\border\s*=\s*([\d.]+)', ib)
            desc_v = re.search(r'\bdesc\s*=\s*([a-z_0-9]+)', ib)
            worlds = None
            if world:
                w = world.group(1)
                worlds = re.findall(r'"([^"]+)"', w) if '{' in w else w  # 变量名（下面代换）
            items[key] = {
                'value': val.group(1) if val else 'default',
                'image': img.group(1) if img else None,
                'world': worlds,
                'order': float(iorder.group(1)) if iorder else None,
                'master': 'master_controlled' in ib,
                'desc_var': desc_v.group(1) if desc_v else None,
            }
        if items:
            out[gname] = items
    return out


IA_WORLDS = {'allworlds': ['forest', 'cave', 'shipwrecked', 'volcanoworld', 'porkland'],
             'iaworlds': ['shipwrecked', 'volcanoworld', 'porkland'],
             'shipwrecked': ['shipwrecked'], 'volcano': ['volcanoworld'],
             'allshipwreckedworlds': ['shipwrecked', 'volcanoworld'],
             'hamlet': ['porkland'], 'oceanworlds': ['forest', 'shipwrecked', 'porkland'],
             'dstworlds': ['forest', 'cave'], 'forest': ['forest'], 'cave': ['cave']}


def parse_cherry():
    """樱林：cherry_customizations.lua 的注册 + init_worldgen.lua 的 world 默认值。"""
    src = _strip(open(os.path.join(CHERRY, 'scripts', 'map', 'cherry_customizations.lua'), encoding='utf-8', errors='replace').read())
    items = {}
    tbl_start = src.index('local customizations = {')
    block, _ = _balanced(src[tbl_start + len('local customizations = {') - 1:])
    for key, ib in _entries(block, r'\b([a-z0-9_]+)\s*=\s*(?=\{)'):
        cm = re.search(r'category\s*=\s*LEVELCATEGORY\.([A-Z]+)', ib)
        if not cm:
            continue
        cat = cm.group(1)
        grp = re.search(r'group\s*=\s*"([a-z_]+)"', ib)
        world = re.search(r'world\s*=\s*(?=\{)([^}]*)\}', ib)
        iorder = re.search(r'\border\s*=\s*([\d.]+)', ib)
        desc_v = re.search(r'\bdesc\s*=\s*"([a-z_0-9]+)"', ib)
        items[key] = {
            'tab': 'WORLDGEN' if cat == 'WORLDGEN' else 'WORLDSETTINGS',
            'group': grp.group(1) if grp else 'misc',
            'world': re.findall(r'"([^"]+)"', world.group(1)) if world else None,  # None → 默认世界
            'order': float(iorder.group(1)) if iorder else None,
            'master': 'master_controlled' in ib,
            'value': 'default',
            'desc_var': desc_v.group(1) if desc_v else 'frequency_descriptions',
        }
    # 消费点：world = v.world or {"forest", "shipwrecked", "volcanoworld"}；image = worldgen_/worldsettings_ + name
    for k, v in items.items():
        if v['world'] is None:
            v['world'] = ['forest', 'shipwrecked', 'volcanoworld']
        v['image'] = ('worldsettings_' if v['tab'] == 'WORLDSETTINGS' else 'worldgen_') + k + '.tex'
        v['atlas'] = 'worldgen_cherry'
    return items


# ---------------------------------------------------------------- 构建
def main():
    po_zh, po_en = load_po_names()
    ia_zh, ia_en = load_ia_names()
    ch_zh, ch_en = load_cherry_names()
    vanilla = parse_customize()
    islands, volcanow = parse_vanilla_whitelists()
    ia_old_gen = parse_reg_table(os.path.join(MODS, '1467214795', 'modservercreationmain.lua'), 'ia_worldgen_customize_table')
    ia_old_set = parse_reg_table(os.path.join(MODS, '1467214795', 'modservercreationmain.lua'), 'ia_settings_customize_table')
    ia_core_set = parse_reg_table(os.path.join(MODS, '3435352667', 'modservercreationmain.lua'), 'ia_settings_customize_table')
    cherry = parse_cherry()

    # 世界名变量代换 + 统一成注册条目
    def fix_world(w):
        if w is None:
            return None
        if isinstance(w, str):
            return IA_WORLDS.get(w, None)
        return w

    registry = {}  # key -> {tab, group, world, master, value, desc_var, image, atlas, mod, mod_id}
    for tab, tables, mod in [('WORLDGEN', [ia_old_gen], 'ia'), ('WORLDSETTINGS', [ia_old_set, ia_core_set], 'ia')]:
        for t in tables:
            for g, items in t.items():
                for key, it in items.items():
                    it = dict(it, tab=tab, group=g, mod=mod)
                    it['world'] = fix_world(it['world'])
                    is_core = mod == 'ia' and t is ia_core_set
                    it['atlas'] = 'customization_core' if is_core else 'customization_shipwrecked'
                    # 模组可用性闸门用：IA 核心 3435352667 / IA 本体 1467214795 / 樱林 1289779251
                    it['mod_id'] = 'workshop-3435352667' if is_core else 'workshop-1467214795'
                    registry[key] = it
    for key, it in cherry.items():
        registry[key] = dict(it, mod='cherry', mod_id='workshop-1289779251')

    # 原版条目（含 IA 白名单扩展）
    vanilla_items = {}
    for tab in ('WORLDGEN', 'WORLDSETTINGS'):
        for cat, c in vanilla[tab].items():
            for key, it in c['items'].items():
                it = dict(it, tab=tab, group=cat, atlas=c['atlas'], desc_var=it['desc_var'] or c['desc_var'], mod='vanilla')
                w = list(it['world']) if it['world'] else None
                if w is not None:
                    if key in islands and 'shipwrecked' not in w:
                        w.append('shipwrecked')
                    if key in volcanow and 'volcanoworld' not in w:
                        w.append('volcanoworld')
                else:
                    w = None
                it['world'] = w
                vanilla_items[(tab, key)] = it

    atlas_cache = {}
    def atlas_of(name):
        if name not in atlas_cache:
            atlas_cache[name] = atlas_meta(name)
        return atlas_cache[name]

    def visible(it, world):
        loc = LOC[world]
        if it['world'] is not None and loc not in it['world']:
            return False
        return True

    # 名称解析（zh/en）
    def names_for(key, mod):
        if mod == 'vanilla':
            return (po_zh.get('STRINGS.UI.CUSTOMIZATIONSCREEN.' + key.upper()),
                    po_en.get('STRINGS.UI.CUSTOMIZATIONSCREEN.' + key.upper()))
        if mod == 'ia':
            return (ia_zh.get(key.upper()), ia_en.get(key.upper()))
        return (ch_zh.get(key.upper()), ch_en.get(key.upper()))

    cur = json.load(open(PUB_JSON, encoding='utf-8'))

    def domain_map(lang, domain, world):
        """值域 {value: label}。优先用现目录同值域，否则由 po 键合成。"""
        keys = DOMAIN_KEYS.get(domain)
        if domain == 'ssw':
            if lang == 'zh':
                return {v: {'SHIPWRECKED_DEFAULT': '温和季', 'HURRICANE': '飓风季', 'MONSOON': '雨季', 'DRY': '旱季',
                            'MILD_MONSOON': '温和季或雨季', 'HURRICANE_DRY': '飓风季或旱季', 'RANDOM': '随机'}[k] for k, v in SW_SEASON_START[1]}
            return {v: {'SHIPWRECKED_DEFAULT': 'Mild', 'HURRICANE': 'Hurricane', 'MONSOON': 'Monsoon', 'DRY': 'Dry',
                        'MILD_MONSOON': 'Mild or Monsoon', 'HURRICANE_DRY': 'Hurricane or Dry', 'RANDOM': 'Random'}[k] for k, v in SW_SEASON_START[1]}
        if not keys:
            return None
        labels = po_zh if lang == 'zh' else po_en
        prefix = 'STRINGS.UI.SANDBOXMENU.'
        out = {}
        for strkey, val in keys:
            lbl = labels.get(prefix + strkey)
            if lbl is None and domain == 'ee':
                lbl = {'SLIDEDEFAULT': '默认', 'SLIDEALWAYS': '总是'}[strkey] if lang == 'zh' else {'SLIDEDEFAULT': 'Default', 'SLIDEALWAYS': 'Always'}[strkey]
            if lbl is None:
                lbl = val
            out[val] = lbl
        return out

    # 从现目录提取 item 级数据（text/desc/value/image UV 均保留，仅归组变化时复用）
    def cur_item(lang, world, tab, key):
        try:
            g = cur[lang][world]['WORLDGEN_GROUP' if tab == 'WORLDGEN' else 'WORLDSETTINGS_GROUP']
            for cat in g.values():
                if key in cat['items']:
                    return cat['items'][key], cat
        except (KeyError, TypeError):
            pass
        return None, None

    result = {lang: {w: {} for w in STRUCT} for lang in ('zh', 'en')}
    reg_out = {}
    stat = []

    for lang in ('zh', 'en'):
        for world, tabs in STRUCT.items():
            for tab, cats in tabs.items():
                group_key = tab + '_GROUP'
                outg = {}
                # 目标条目集合：静态（原版 + 海难/火山的 IA）
                want = {}
                for (t, key), it in vanilla_items.items():
                    if t != tab or not visible(it, world):
                        continue
                    want[key] = it
                if world in ('shipwrecked', 'volcano'):
                    for key, it in registry.items():
                        if it['tab'] != tab or it['mod'] != 'ia' or not visible(it, world):
                            continue
                        want[key] = it
                for cat_id, cat_zh, cat_en, cat_order in cats:
                    citems = {}
                    for key, it in want.items():
                        if it['group'] != cat_id:
                            continue
                        data, prev_cat = cur_item(lang, world, tab, key)
                        azh, aen = names_for(key, it['mod'])
                        text = (data or {}).get('text') or (azh if lang == 'zh' else aen) or key
                        # 值域
                        dvar = it.get('desc_var')
                        domain = DESCVAR2DOMAIN.get(dvar) or CAT_DOMAIN.get(tab, {}).get(cat_id, 'yesno')
                        desc = (data or {}).get('desc') or domain_map(lang, domain, world)
                        value = it['value']
                        if desc and value not in desc:
                            value = list(desc)[0] if desc else value
                        # 图标 UV：按注册图集取；现目录已有 UV 且同图集则复用
                        am = atlas_of(it['atlas'])
                        img_name = it['image']
                        uv = am['items'].get(img_name)
                        atlas_meta_out = {'name': am['name'], 'width': am['width'], 'height': am['height'], 'item_size': am['item_size']}
                        if uv is None:
                            uv = (data or {}).get('image')
                            atlas_meta_out = (prev_cat or {}).get('atlas') or atlas_meta_out
                        entry = {'image': uv, 'text': text, 'value': value, '_atlas': atlas_meta_out}
                        if desc:
                            entry['desc'] = desc
                        if it.get('order') is not None:
                            entry['order'] = it['order']
                        if it['master']:
                            entry['master'] = True
                        citems[key] = entry
                    # 排序：显式 order 优先，之后按显示名（与游戏 stringidsorter 同效）
                    def sort_key(kv):
                        e = kv[1]
                        return (e.get('order') is None, e.get('order') or 0, e.get('text') or '')
                    citems = dict(sorted(citems.items(), key=sort_key))
                    if citems:
                        # 分类图集取条目里出现最多的（图标混合图集时 per-item atlas 兜底）
                        cnt = {}
                        for e in citems.values():
                            n = e['_atlas']['name']
                            cnt[n] = cnt.get(n, 0) + 1
                        main_atlas = max(cnt, key=cnt.get)
                        am = atlas_of(main_atlas)
                        for e in citems.values():
                            ia_ = e.pop('_atlas')
                            if ia_['name'] != main_atlas:
                                e['atlas'] = ia_
                        outg[cat_id] = {'order': cat_order, 'text': cat_zh if lang == 'zh' else cat_en,
                                        'atlas': {'name': am['name'], 'width': am['width'], 'height': am['height'], 'item_size': am['item_size']},
                                        'desc': domain_map(lang, CAT_DOMAIN.get(tab, {}).get(cat_id, 'yesno'), world),
                                        'items': citems}
                result[lang][world][group_key] = outg
                stat.append((lang, world, tab, {c: len(outg[c]['items']) for c in outg}))

    # 游戏不展示的杂项键（WORLDGEN_MISC/WORLDSETTINGS_MISC 家族 + 预设书签等），渲染端直接隐藏
    hidden_keys = set()
    z2 = zipfile.ZipFile(os.path.join(GAME_DATA, r'databundles\scripts.zip'))
    src2 = _strip(z2.read('scripts/map/customize.lua').decode('utf-8', 'replace'))
    for fam in ('WORLDGEN_MISC', 'WORLDSETTINGS_MISC'):
        m = re.search(r'local %s = \{(.*?)\}' % fam, src2, re.S)
        if m:
            hidden_keys |= set(re.findall(r'"([a-z0-9_]+)"', m.group(1)))

    # 运行时注册表（森林/洞穴等静态目录之外的模组项；渲染端：文件里有该键才显示）
    for key, it in registry.items():
        am = atlas_of(it['atlas'])
        entry = {
            'tab': it['tab'], 'group': it['group'], 'world': it['world'], 'master': bool(it['master']),
            'value': it['value'], 'image': it['image'], 'atlas': {'name': am['name'], 'width': am['width'], 'height': am['height'], 'item_size': am['item_size']},
            'uv': am['items'].get(it['image']),
            'descDomain': DESCVAR2DOMAIN.get(it.get('desc_var')) or CAT_DOMAIN.get(it['tab'], {}).get(it['group'], 'freq5'),
            'modId': it.get('mod_id') or 'workshop-1467214795',
        }
        zh_n, en_n = names_for(key, it['mod'])
        entry['text'] = {'zh': zh_n or key, 'en': en_n or key}
        if it.get('order') is not None:
            entry['order'] = it['order']
        reg_out[key] = entry

    os.makedirs(os.path.dirname(REG_JSON), exist_ok=True)
    domains = {}
    for dom in set(e['descDomain'] for e in reg_out.values()):
        domains[dom] = {'zh': domain_map('zh', dom, 'forest') or {}, 'en': domain_map('en', dom, 'forest') or {}}
    json.dump({'hiddenKeys': sorted(hidden_keys), 'domains': domains, 'options': reg_out}, io.open(REG_JSON, 'w', encoding='utf-8'), ensure_ascii=False, indent=1)

    # 对账打印（截图口径）
    expect = {
        ('zh', 'forest', 'WORLDSETTINGS'): {'global': 13, 'events': 13, 'survivors': 11, 'misc': 15, 'resources': 12, 'portal_resources': 6, 'animals': 15, 'monsters': 14, 'giants': 16, 'lunar_mutations': 11},
        ('zh', 'forest', 'WORLDGEN'): {'global': 1, 'misc': 14, 'resources': 26, 'animals': 13, 'monsters': 10},
        ('zh', 'cave', 'WORLDSETTINGS'): {'global': 13, 'events': 13, 'survivors': 11, 'misc': 8, 'resources': 10, 'animals': 11, 'monsters': 13, 'giants': 5, 'lunar_mutations': 4},
        ('zh', 'cave', 'WORLDGEN'): {'global': 1, 'misc': 9, 'resources': 17, 'animals': 5, 'monsters': 7},
        ('zh', 'shipwrecked', 'WORLDSETTINGS'): {'global': 18, 'events': 13, 'survivors': 11, 'misc': 15, 'resources': 17, 'animals': 18, 'monsters': 11, 'giants': 11},
        ('zh', 'shipwrecked', 'WORLDGEN'): {'global': 2, 'misc': 12, 'resources': 33, 'animals': 14, 'monsters': 9},
        ('zh', 'volcano', 'WORLDSETTINGS'): {'global': 18, 'events': 13, 'survivors': 11, 'misc': 5, 'resources': 6, 'animals': 7, 'monsters': 5, 'giants': 4},
        ('zh', 'volcano', 'WORLDGEN'): {'global': 2, 'misc': 2, 'resources': 6, 'monsters': 1},
    }
    print('== 静态目录条目数对账（zh，期望=截图口径）==')
    ok = True
    for lang, world, tab, counts in stat:
        if lang != 'zh':
            continue
        exp = expect.get((lang, world, tab))
        mark = ''
        if exp and counts != exp:
            mark = '  <<< 不符 期望=%s' % exp
            ok = False
        print('%-12s %-13s %s%s' % (world, tab, counts, mark))
    print('对账', 'OK' if ok else '存在差异')
    print('注册表条目:', len(reg_out))

    if '--write' in os.sys.argv:
        json.dump(result, io.open(PUB_JSON, 'w', encoding='utf-8'), ensure_ascii=False)
        print('已写入', PUB_JSON)
    else:
        json.dump(result, io.open(PUB_JSON + '.new', 'w', encoding='utf-8'), ensure_ascii=False)
        print('预览写入', PUB_JSON + '.new  （加 --write 覆盖正式文件）')


if __name__ == '__main__':
    main()
