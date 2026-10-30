"""Exercise release Compose in a disposable project; never touch the developer volume."""
import argparse
import json
from pathlib import Path
import secrets
import socket
import subprocess
import tempfile
import time
import urllib.error
import urllib.request

ROOT = Path(__file__).resolve().parents[1]

def smoke(image, platform=None):
    project = 'observatory-release-check-' + secrets.token_hex(6)
    with socket.socket() as sock:
        sock.bind(('127.0.0.1',0)); port = sock.getsockname()[1]
    with tempfile.TemporaryDirectory(prefix='observatory-release-check-') as tmp:
        folder = Path(tmp)
        envfile = folder/'.env'
        envfile.write_text(f'OBSERVATORY_IMAGE={image}\nPOSTGRES_PASSWORD={secrets.token_hex(24)}\nOBSERVATORY_PORT={port}\n',encoding='utf-8')
        override = folder/'override.json'
        override.write_text(json.dumps({'services':{'app':{'platform':platform} if platform else {}}}),encoding='utf-8')
        base = ['docker','compose','--project-directory',str(folder),'--env-file',str(envfile),'-f',str(ROOT/'packaging/compose.yaml'),'-f',str(override),'-p',project]
        def compose(*args, capture=False):
            return subprocess.run(base+list(args),check=True,text=True,encoding='utf-8',stdout=subprocess.PIPE if capture else None).stdout
        def ready():
            for _ in range(60):
                try:
                    with urllib.request.urlopen(f'http://127.0.0.1:{port}/api/health',timeout=3) as r:
                        value=json.load(r)
                    if value['database_ok'] and value['inventory_fresh']: return
                except (OSError, urllib.error.URLError): pass
                time.sleep(1)
            raise RuntimeError('Release app did not become ready')
        try:
            compose('config','--quiet')
            compose('up','-d')
            ready()
            with urllib.request.urlopen(f'http://127.0.0.1:{port}/') as r:
                assert 'Observatory' in r.read().decode()
            compose('exec','-T','app','/observatory','--version')
            # Verify automatic migrations and persistence across a stop/recreate.
            count=compose('exec','-T','db','psql','-U','observatory','-d','observatory','-Atc','SELECT count(*) FROM schema_migrations',capture=True).strip()
            assert count=='5', count
            compose('exec','-T','db','psql','-U','observatory','-d','observatory','-v','ON_ERROR_STOP=1','-c',"CREATE TABLE release_probe(value text); INSERT INTO release_probe VALUES('preserved')")
            compose('stop','app')
            compose('exec','-T','db','pg_dump','-U','observatory','-d','observatory','-Fc','-f','/tmp/observatory.dump')
            dump=folder/'observatory.dump'
            compose('cp','db:/tmp/observatory.dump',str(dump))
            assert dump.stat().st_size>0
            compose('exec','-T','db','createdb','-U','observatory','restore_check')
            compose('exec','-T','db','pg_restore','--exit-on-error','--no-owner','-U','observatory','-d','restore_check','/tmp/observatory.dump')
            restored=compose('exec','-T','db','psql','-U','observatory','-d','restore_check','-Atc','SELECT count(*) FROM schema_migrations',capture=True).strip()
            assert restored==count
            assert compose('exec','-T','db','psql','-U','observatory','-d','restore_check','-Atc','SELECT value FROM release_probe',capture=True).strip()=='preserved'
            compose('up','-d','--force-recreate','app')
            ready()
            assert compose('exec','-T','db','psql','-U','observatory','-d','observatory','-Atc','SELECT value FROM release_probe',capture=True).strip()=='preserved'
            print('PASS: release readiness, dashboard, version, five migrations, binary-safe backup/restore and app recreation ('+(platform or 'native')+')')
        finally:
            # The only deletion is this random, owned, disposable project's data.
            compose('down','--volumes')

if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('--image',required=True)
    p.add_argument('--platform')
    args=p.parse_args()
    smoke(args.image,args.platform)
