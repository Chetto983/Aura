package usersandbox

// dir_fd + O_NOFOLLOW pin the destination directory; O_TMPFILE keeps partial data
// unnamed. linkat through /proc/self/fd publishes that exact inode without replacing
// any existing name. See docs.python.org/3/library/os.html#os.link and open(2).
const browserDownloadReceiveScript = `import os,sys,json
name,size=sys.argv[1],int(sys.argv[2])
root=os.open('/workspace',os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW)
try: os.mkdir('downloads',0o755,dir_fd=root)
except FileExistsError: pass
d=os.open('downloads',os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW,dir_fd=root)
f=os.open('.',os.O_RDWR|os.O_TMPFILE,0o644,dir_fd=d)
with os.fdopen(f,'wb',closefd=False) as out:
    remaining=size
    while remaining:
        data=sys.stdin.buffer.read(min(65536,remaining))
        if not data: raise RuntimeError('truncated download')
        out.write(data)
        remaining-=len(data)
    if sys.stdin.buffer.read(1): raise RuntimeError('oversized download')
    out.flush()
    os.fsync(f)
    stem,ext=os.path.splitext(name)
    for n in range(1,10001):
        target=name if n==1 else '%s-%d%s'%(stem,n,ext)
        try: os.link('/proc/self/fd/%d'%f,target,dst_dir_fd=d,follow_symlinks=True)
        except FileExistsError: continue
        os.fsync(d)
        print(json.dumps('/workspace/downloads/'+target))
        break
    else: raise RuntimeError('download name space exhausted')
os.close(f)
os.close(d)
os.close(root)
`

const browserDownloadListScript = `import os,json,stat
root='/workspace/downloads'
files=[]
if os.path.lexists(root):
    d=os.open(root,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW)
    with os.scandir(d) as entries:
        for entry in entries:
            s=entry.stat(follow_symlinks=False)
            if stat.S_ISREG(s.st_mode) and s.st_nlink==1:
                files.append(dict(name=entry.name,size=s.st_size,mtime=s.st_mtime_ns))
                if len(files)>1000: raise RuntimeError('too many downloads')
    os.close(d)
try:
    with open('/workspace/.aura-downloads.json','rb') as f:
        data=f.read((1<<20)+1)
        if len(data)>1<<20: raise RuntimeError('download checkpoint too large')
        delivered=json.loads(data)
except FileNotFoundError: delivered={}
print(json.dumps(dict(files=files,delivered=delivered)))
`

// Replace only a fully written, validated checkpoint; a failed or killed copy
// leaves the previous manifest readable. os.replace is atomic within one volume.
const browserDownloadCheckpointScript = `import os,sys,json
d=os.open('/workspace',os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW)
name=sys.argv[1]
f=os.open(name,os.O_RDONLY|os.O_NOFOLLOW,dir_fd=d)
with os.fdopen(f,'rb') as source:
    data=source.read((1<<20)+1)
    if len(data)>1<<20: raise RuntimeError('download checkpoint too large')
    if not isinstance(json.loads(data),dict): raise RuntimeError('invalid download checkpoint')
    os.fsync(source.fileno())
os.replace(name,'.aura-downloads.json',src_dir_fd=d,dst_dir_fd=d)
os.fsync(d)
os.close(d)
`
