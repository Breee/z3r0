import os, pty, sys, time, select, re
keys = sys.argv[1].encode().decode('unicode_escape').split('|')
pid, fd = pty.fork()
if pid == 0:
    os.environ['TERM'] = 'xterm-256color'
    os.execvp(sys.argv[2], sys.argv[2:])
out = b''
def drain(t):
    global out
    end = time.time() + t
    while time.time() < end:
        r, _, _ = select.select([fd], [], [], 0.1)
        if r:
            try: out += os.read(fd, 65536)
            except OSError: return
drain(1.5)
for k in keys:
    os.write(fd, k.encode()); drain(0.7)
drain(1.0)
_, st = os.waitpid(pid, 0)
txt = re.sub(rb'\x1b\[[0-9;?<>=]*[a-zA-Z~]|\x1b[\]P_][^\x07\x1b]*(\x07|\x1b\\)|\x1b[=>]', b'', out).decode(errors='replace')
lines = [l.rstrip() for l in txt.replace('\r', '\n').split('\n') if l.strip()]
print('\n'.join(lines[-int(os.environ.get('TAIL', '25')):]))
print('rc', os.waitstatus_to_exitcode(st))
