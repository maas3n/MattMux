import json, re, subprocess, time, traceback
from pathlib import Path
from xml.etree import ElementTree as ET

OUT = Path('evidence'); OUT.mkdir(exist_ok=True)
ROOT = '/sdcard/Download/MattMuxTest'
PKG = 'io.github.maas3n.mattmux'

def adb(*args, check=True):
    return subprocess.run(['adb', *args], check=check, capture_output=True, text=True, timeout=45).stdout

def dump():
    adb('shell', 'uiautomator', 'dump', '/sdcard/window.xml')
    xml = adb('shell', 'cat', '/sdcard/window.xml')
    (OUT / 'last-ui.xml').write_text(xml)
    return ET.fromstring(xml)

def tap_node(n):
    x1,y1,x2,y2 = map(int,re.findall(r'\d+',n.attrib['bounds']))
    adb('shell','input','tap',str((x1+x2)//2),str((y1+y2)//2)); time.sleep(.6)

def click(label, scroll=False, optional=False):
    for attempt in range(8 if scroll else 4):
        tree = dump()
        for n in tree.iter('node'):
            if label.casefold() in (n.get('text','').casefold(), n.get('content-desc','').casefold()):
                if n.get('enabled') != 'true':
                    continue
                tap_node(n); return True
        if scroll:
            adb('shell','input','swipe','350','1050','350','350','350')
        time.sleep(.5)
    if optional: return False
    raise AssertionError('UI item missing: '+label)

def top():
    for _ in range(3): adb('shell','input','swipe','350','350','350','1050','200')

def select_path(button, parts, folder):
    top(); click(button)
    click('Show roots')
    click('Downloads')
    click('MattMuxTest')
    for part in parts: click(part,scroll=True)
    if folder:
        click('Use this folder')
        click('Allow',optional=True)

def screenshot(name):
    result = subprocess.run(['adb','exec-out','screencap','-p'],capture_output=True,check=True)
    (OUT/name).write_bytes(result.stdout)

def verify_selection(source, destination):
    top()
    texts = ' '.join(n.get('text','') for n in dump().iter('node'))
    assert source in texts and destination in texts, texts

def run():
    adb('install','-r','alpha4.apk')
    adb('shell','mkdir','-p',ROOT+'/out-folder',ROOT+'/out-iso')
    adb('push','fixture/disc',ROOT+'/disc')
    adb('push','fixture/disc.iso',ROOT+'/disc.iso')
    adb('logcat','-c')
    adb('shell','am','start','-W','-n',PKG+'/.MainActivity')
    time.sleep(2)
    screenshot('android-startup.png')
    for kind in ('folder','iso'):
        source = 'disc' if kind == 'folder' else 'disc.iso'
        select_path('Choose DVD folder' if kind=='folder' else 'Choose ISO',[source],kind=='folder')
        select_path('Choose output folder',['out-'+kind],True)
        verify_selection(source,'out-'+kind)
        adb('shell','wm','size','800x1200')
        time.sleep(2)
        verify_selection(source,'out-'+kind)
        screenshot('android-'+kind+'-resized.png')
        adb('shell','wm','size','reset')
        time.sleep(2)
        verify_selection(source,'out-'+kind)
        click('Remux to MKV',scroll=True)
        deadline=time.monotonic()+90
        while time.monotonic()<deadline:
            texts=' '.join(n.get('text','') for n in dump().iter('node'))
            if 'Remux failed:' in texts: raise AssertionError(texts)
            if 'Complete: title' in texts: break
            time.sleep(1)
        else: raise AssertionError('Remux did not complete')
        screenshot('android-'+kind+'-complete.png')
        paths=adb('shell','find',ROOT+'/out-'+kind,'-name','*.mkv').splitlines()
        assert len(paths)==1,paths
        adb('pull',paths[0],str(OUT/('android-'+kind+'.mkv')))
        assert (OUT/('android-'+kind+'.mkv')).stat().st_size>0
        print('Published Android APK '+kind+' selection/resize/remux PASS',flush=True)
    subprocess.run(['python3','audit/compare-output.py',str(OUT/'android-folder.mkv'),str(OUT/'android-iso.mkv')],check=True)

try:
    run()
except Exception:
    screenshot('android-failure.png')
    (OUT/'android-error.txt').write_text(traceback.format_exc())
    raise
finally:
    (OUT/'android-logcat.txt').write_text(adb('logcat','-d',check=False))
    (OUT/'android-device.txt').write_text(adb('shell','getprop',check=False))
