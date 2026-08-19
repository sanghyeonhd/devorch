import os, subprocess, tempfile, sqlite3, json, shutil, pathlib, textwrap, time
from dataclasses import dataclass

root = tempfile.mkdtemp(prefix='devorch-sim-')
repo = pathlib.Path(root)/'repo'
subprocess.run(['git','init','-b','main',str(repo)], check=True, capture_output=True, text=True)
subprocess.run(['git','-C',str(repo),'config','user.email','sim@example.com'], check=True)
subprocess.run(['git','-C',str(repo),'config','user.name','DevOrch Simulation'], check=True)
(repo/'.gitignore').write_text('__pycache__/\n*.pyc\n', encoding='utf-8')
(repo/'calculator.py').write_text('def add(a, b):\n    return a - b  # seeded bug\n', encoding='utf-8')
(repo/'test_calculator.py').write_text(textwrap.dedent('''\
import unittest
from calculator import add

class T(unittest.TestCase):
    def test_add_positive(self): self.assertEqual(add(2, 3), 5)
    def test_add_negative(self): self.assertEqual(add(-2, 3), 1)
    def test_add_zero(self): self.assertEqual(add(0, 4), 4)

if __name__ == '__main__': unittest.main()
'''), encoding='utf-8')
subprocess.run(['git','-C',str(repo),'add','.'], check=True)
subprocess.run(['git','-C',str(repo),'commit','-m','seed bug'], check=True, capture_output=True)
base_sha = subprocess.check_output(['git','-C',str(repo),'rev-parse','HEAD'], text=True).strip()

def test(cwd):
    p = subprocess.run(['python','-m','unittest','-q'],cwd=cwd,text=True,capture_output=True)
    return p.returncode, (p.stdout+p.stderr).strip()

baseline_rc, baseline_out = test(repo)

agents = ['codex','claude','grok','gemini']
worktrees = {}
for a in agents:
    wt = pathlib.Path(root)/f'wt-{a}'
    subprocess.run(['git','-C',str(repo),'worktree','add','-b',f'sim/{a}',str(wt),base_sha], check=True, capture_output=True)
    worktrees[a]=wt

# mock patches: three pass, one fails; passing candidates have different diff sizes
(worktrees['codex']/'calculator.py').write_text('def add(a, b):\n    return a + b\n', encoding='utf-8')
(worktrees['claude']/'calculator.py').write_text(textwrap.dedent('''\
def add(a, b):
    """Return the arithmetic sum of two numeric values."""
    return a + b
'''), encoding='utf-8')
(worktrees['grok']/'calculator.py').write_text('def add(a, b):\n    return a * b\n', encoding='utf-8')
(worktrees['gemini']/'calculator.py').write_text('def add(a, b):\n    # fixed seeded regression\n    return a + b\n', encoding='utf-8')
(worktrees['gemini']/'NOTES.md').write_text('Gemini candidate touched an unnecessary file.\n', encoding='utf-8')

results={}
for a,wt in worktrees.items():
    rc,out=test(wt)
    diff=subprocess.check_output(['git','-C',str(wt),'diff','--numstat'], text=True).strip().splitlines()
    untracked=subprocess.check_output(['git','-C',str(wt),'ls-files','--others','--exclude-standard'], text=True).strip().splitlines()
    tracked_changed=sum(1 for x in diff if x.strip())
    changed=tracked_changed+sum(1 for x in untracked if x.strip())
    churn=0
    for line in diff:
        parts=line.split('\t')
        if len(parts)>=2 and parts[0].isdigit() and parts[1].isdigit(): churn += int(parts[0])+int(parts[1])
    for rel in untracked:
        if rel.strip():
            try: churn += len((wt/rel).read_text(encoding='utf-8').splitlines())
            except Exception: churn += 1000000
    results[a]={'tests_pass':rc==0,'changed_files':changed,'untracked_files':untracked,'churn':churn,'test_output':out}
    subprocess.run(['git','-C',str(wt),'add','.'], check=True)
    subprocess.run(['git','-C',str(wt),'commit','-m',f'{a} candidate'], check=True, capture_output=True)
    results[a]['commit']=subprocess.check_output(['git','-C',str(wt),'rev-parse','HEAD'],text=True).strip()

passing=[a for a,r in results.items() if r['tests_pass']]
winner=min(passing,key=lambda a:(results[a]['changed_files'],results[a]['churn'],a))

# integration branch and merge winner
subprocess.run(['git','-C',str(repo),'checkout','-b','sim/integration'],check=True,capture_output=True)
subprocess.run(['git','-C',str(repo),'merge','--no-ff',f'sim/{winner}','-m',f'merge {winner} winner'],check=True,capture_output=True)
final_rc, final_out=test(repo)
final_sha=subprocess.check_output(['git','-C',str(repo),'rev-parse','HEAD'],text=True).strip()

# SQLite persistence/restart simulation
mdb=pathlib.Path(root)/'mission.db'
conn=sqlite3.connect(mdb)
conn.execute('create table mission(id text primary key, state text, winner text, base_sha text, final_sha text)')
conn.execute('create table candidate(agent text primary key, tests_pass int, changed_files int, churn int, commit_sha text)')
conn.execute('insert into mission values(?,?,?,?,?)',('m1','SUCCEEDED',winner,base_sha,final_sha))
for a,r in results.items(): conn.execute('insert into candidate values(?,?,?,?,?)',(a,int(r['tests_pass']),r['changed_files'],r['churn'],r['commit']))
conn.commit(); conn.close()
conn=sqlite3.connect(mdb)
row=conn.execute('select state,winner,base_sha,final_sha from mission where id=?',('m1',)).fetchone()
rows=conn.execute('select agent,tests_pass,changed_files,churn from candidate order by agent').fetchall()
conn.close()
persistence_ok = row[0]=='SUCCEEDED' and row[1]==winner and len(rows)==4

# common event normalization simulation using documented/public shapes
samples={
'gemini':[
 {'type':'init','session_id':'g1','model':'gemini','timestamp':'t'},
 {'type':'message','role':'assistant','content':'working','timestamp':'t'},
 {'type':'tool_use','tool_name':'write_file','tool_id':'x','parameters':{},'timestamp':'t'},
 {'type':'tool_result','tool_id':'x','status':'success','output':'ok','timestamp':'t'},
 {'type':'result','status':'success','stats':{'total_tokens':10},'timestamp':'t'},
],
'grok':[
 {'type':'tool_call','toolCallId':'x','title':'Edit','kind':'edit','status':'in_progress','toolName':'search_replace'},
 {'type':'tool_call_update','toolCallId':'x','status':'completed','rawOutput':{}},
 {'type':'text','data':'done'},
 {'type':'end','stopReason':'end_turn','sessionId':'x','usage':{'input_tokens':1,'output_tokens':1}},
],
'codex':[
 {'type':'thread.started','thread_id':'c1'},
 {'type':'turn.started'},
 {'type':'item.started','item':{'id':'i1','type':'command_execution','command':'test','aggregated_output':'','exit_code':None,'status':'in_progress'}},
 {'type':'item.completed','item':{'id':'i1','type':'command_execution','command':'test','aggregated_output':'ok','exit_code':0,'status':'completed'}},
 {'type':'turn.completed','usage':{'input_tokens':1,'cached_input_tokens':0,'cache_write_input_tokens':0,'output_tokens':1,'reasoning_output_tokens':0}},
],
'claude':[
 {'type':'system','subtype':'init','session_id':'a1'},
 {'type':'assistant','message':{'content':[{'type':'text','text':'working'}]}},
 {'type':'user','message':{'content':[{'type':'tool_result','tool_use_id':'x','content':'ok'}]}},
 {'type':'result','subtype':'success','session_id':'a1'},
]}

def normalize(agent,e):
    t=e.get('type')
    if agent=='gemini':
        return {'init':'SESSION_STARTED','message':'TEXT','tool_use':'TOOL_STARTED','tool_result':'TOOL_COMPLETED','result':'COMPLETED','error':'FAILED'}.get(t,'OTHER')
    if agent=='grok':
        return {'tool_call':'TOOL_STARTED','tool_call_update':'TOOL_COMPLETED','text':'TEXT','end':'COMPLETED','error':'FAILED'}.get(t,'OTHER')
    if agent=='codex':
        return {'thread.started':'SESSION_STARTED','turn.started':'TURN_STARTED','turn.completed':'COMPLETED','turn.failed':'FAILED','item.started':'ITEM_STARTED','item.updated':'ITEM_UPDATED','item.completed':'ITEM_COMPLETED','error':'FAILED'}.get(t,'OTHER')
    if agent=='claude':
        if t=='system' and e.get('subtype')=='init': return 'SESSION_STARTED'
        return {'assistant':'TEXT','user':'TOOL_RESULT','result':'COMPLETED'}.get(t,'OTHER')

normalized={a:[normalize(a,e) for e in evs] for a,evs in samples.items()}
adapter_ok=all('COMPLETED' in xs for xs in normalized.values()) and all(('SESSION_STARTED' in xs) or a=='grok' for a,xs in normalized.items())

# policy simulation
policy_actions=[('read_file','L0'),('edit_worktree','L2'),('git_push','L3'),('delete_outside_worktree','L5')]
def decision(level):
    return {'L0':'ALLOW','L1':'ALLOW','L2':'ALLOW_SCOPE','L3':'ASK','L4':'ASK_STRONG','L5':'DENY'}[level]
policy={a:decision(l) for a,l in policy_actions}
policy_ok=policy['git_push']=='ASK' and policy['delete_outside_worktree']=='DENY'

# efficiency scheduler simulation: avoid keeping all four CLIs alive unless risk requires it.
# low: 1 candidate, medium: 2 candidates, high: 4 candidates; max parallelism 2 by default.
scheduler_cases={
 'low_risk': {'candidates':['codex'], 'max_parallel':1},
 'medium_risk': {'candidates':['codex','claude'], 'max_parallel':2},
 'high_risk': {'candidates':['codex','claude','grok','gemini'], 'max_parallel':2},
}
for v in scheduler_cases.values():
    v['peak_agent_processes']=min(len(v['candidates']),v['max_parallel'])
    v['agents_spawned']=len(v['candidates'])
resource_policy_ok=(scheduler_cases['medium_risk']['peak_agent_processes']==2 and scheduler_cases['high_risk']['peak_agent_processes']==2)

summary={
 'root':root,
 'baseline_expected_failure':baseline_rc!=0,
 'candidate_results':results,
 'winner':winner,
 'final_tests_pass':final_rc==0,
 'persistence_restart_ok':persistence_ok,
 'adapter_normalization_ok':adapter_ok,
 'normalized_events':normalized,
 'policy_ok':policy_ok,
 'policy':policy,
 'scheduler_cases': scheduler_cases,
 'resource_policy_ok': resource_policy_ok,
 'overall_functional_simulation_pass': all([baseline_rc!=0, final_rc==0, persistence_ok, adapter_ok, policy_ok, resource_policy_ok])
}
print(json.dumps(summary,indent=2))
