"""Fork-only exact-head Windows investigation; does not edit product source."""
import collections,datetime,json,os,pathlib,platform,subprocess,sys,time
ROOT=pathlib.Path(sys.argv[1]).resolve(); OUT=pathlib.Path(sys.argv[2]).resolve(); OUT.mkdir(parents=True,exist_ok=True)
SHA='f011c2b0c32241602e9d84691b8b8f1ba84ac4bc'
BASE='d8db233cf9f64bc43bd8b25f7923055260c36762'
results=[]
def read(args,cwd=ROOT): return subprocess.check_output(args,cwd=cwd,text=True).strip()
def run(label,args,cwd=ROOT/'backend',expected_tests=None):
    started=datetime.datetime.now(datetime.timezone.utc).isoformat(); tic=time.monotonic()
    with (OUT/(label+'.log')).open('wb') as log:
        try: code=subprocess.run(args,cwd=cwd,stdout=log,stderr=subprocess.STDOUT,timeout=360).returncode
        except subprocess.TimeoutExpired: code=124
    events=[]
    for line in (OUT/(label+'.log')).read_text(errors='replace').splitlines():
        try: event=json.loads(line)
        except ValueError: continue
        if event.get('Test') and event.get('Action') in ['pass','fail','skip']:
            events.append({k:event[k] for k in ['Package','Test','Action','Elapsed'] if k in event})
    counts=collections.Counter(e['Action'] for e in events)
    seen=collections.Counter(e['Test'] for e in events if e['Action'] in ['pass','fail'])
    selection_ok=bool(events) and all(seen[name]==count for name,count in (expected_tests or {}).items())
    record={'label':label,'argv':args,'cwd':str(cwd),'head':read(['git','rev-parse','HEAD'],cwd),'started_at':started,'duration_seconds':time.monotonic()-tic,'exit_code':code,'expected_tests':expected_tests,'selection_ok':selection_ok,'counts':dict(counts),'test_events':events,'log':label+'.log'}
    results.append(record); (OUT/'results.json').write_text(json.dumps(results,indent=2)+'\n')
    print(json.dumps({k:record[k] for k in ['label','head','duration_seconds','exit_code','selection_ok','counts']}),flush=True)
    return record
assert read(['git','rev-parse','HEAD'])==SHA
assert not read(['git','status','--porcelain=v1'])
assert read(['go','env','GOOS'])=='windows'
identity={'candidate_sha':SHA,'base_sha':BASE,'workflow_sha':os.environ.get('GITHUB_SHA'),'os':platform.platform(),'go_version':read(['go','version']),'goos':read(['go','env','GOOS']),'goarch':read(['go','env','GOARCH']),'image_os':os.environ.get('ImageOS'),'image_version':os.environ.get('ImageVersion'),'gorace':os.environ.get('GORACE','<unset>'),'run_id':os.environ.get('GITHUB_RUN_ID'),'run_attempt':os.environ.get('GITHUB_RUN_ATTEMPT')}
(OUT/'identity.json').write_text(json.dumps(identity,indent=2)+'\n'); print(json.dumps(identity),flush=True)
checksum=ROOT/'go.work.sum'; checksum_bytes=checksum.read_bytes()
try:
    target='TestPersistentClaudeACPHibernateAndNativeResume'
    codex='TestCodexHibernateStopsAppServerAndNativeResumesThread'
    run('candidate-acp-alone-20',['go','test','-race','-json','-count=20','-timeout=240s','-run','^'+target+'$','./internal/adapters/chatdriver/acp/'],expected_tests={target:20})
    run('candidate-official-pair-10',['go','test','-race','-json','-count=10','-timeout=240s','-run','^('+target+'|'+codex+')$','./internal/adapters/chatdriver/acp/','./internal/adapters/chatdriver/codexappserver/'],expected_tests={target:10,codex:10})
    run('candidate-shutdown-10',['go','test','-race','-json','-count=10','-timeout=240s','-run','^Test(Shutdown.*|WindowsProviderJobReapsGrandchild)$','./internal/adapters/chatdriver/persistenthost/'],expected_tests={'TestShutdownReleasesHostBeforeFreshReplacement':10,'TestShutdownHonorsContextAfterDial':10,'TestShutdownPreservesUnknownOwnerButAcceptsConclusiveDeath':10,'TestShutdownTimesOutSilentLiveHost':10,'TestWindowsProviderJobReapsGrandchild':10})
    run('candidate-acp-model-10',['go','test','-race','-json','-count=10','-timeout=120s','-run','^TestModernACPModel','./internal/adapters/chatdriver/acp/'])
    base=ROOT.parent/'base-control'
    subprocess.run(['git','worktree','add','--detach',str(base),BASE],cwd=ROOT,check=True)
    relevant=['backend/internal/adapters/chatdriver/acp/restart_test.go','backend/internal/adapters/chatdriver/acp/process.go','backend/internal/adapters/chatdriver/persistenthost/host.go','backend/internal/adapters/chatdriver/persistenthost/child_windows.go']
    same={f:subprocess.check_output(['git','rev-parse',SHA+':'+f],cwd=ROOT)==subprocess.check_output(['git','rev-parse',BASE+':'+f],cwd=ROOT) for f in relevant}
    (OUT/'base-source-equality.json').write_text(json.dumps(same,indent=2)+'\n')
    assert all(same.values())
    run('base-acp-alone-10',['go','test','-race','-json','-count=10','-timeout=180s','-run','^'+target+'$','./internal/adapters/chatdriver/acp/'],cwd=base/'backend',expected_tests={target:10})
finally:
    (OUT/'candidate-after-suites.diff').write_bytes(subprocess.check_output(['git','diff'],cwd=ROOT))
    if checksum.read_bytes()!=checksum_bytes:
        (OUT/'candidate-after-suites-go-work-sum').write_bytes(checksum.read_bytes())
        checksum.write_bytes(checksum_bytes)
    status=read(['git','status','--porcelain=v1'])
    (OUT/'candidate-final-status.txt').write_text(status+'\n')
    assert not status,status
    assert read(['git','rev-parse','HEAD'])==SHA
assert all(r['exit_code']==0 and r['selection_ok'] for r in results), 'Executed failure or selection failure: inspect artifact, not an automatic fix'
