# engwrap
**engwrap** is a small and simple wrapper around Docker that allow you to run various environment, each with full command and output logging.
Actually this project started for a simple reason: first because I prefero to use docker containers as a base for my red team activities, and second... **I want to know exactly what I did, when I did it, and in which environment**, even weeks later, when my memory has betrayed me completely.

## why
If OPSEC is about managing risk, the first risk to manage is usual... ourselves.

What I mean is simple: before worrying about stealth, evasion, or whatever new trick shows up on GitHub, we should at least make sure our own environment isn't actively sabotaging us.
In fact, most operational failures are often self-inflicted: lost tools, undocumented changes, and a fuzzy memory of our own actions.

So `engwrap` was born to "_solve_" exactly that: a tool that help me manage environments giving each one a clean, isolated workspace where every action is **logged**, every command is **recorded**, and everything is under control.

*It's just a way to avoid shooting yourself in the foot before the operation even starts.*

## usage

```bash
   (       (  (  (  (   (      )        
  ))\ (    )\))( )\))(  )(  ( /( \`  )   
 /((_))\ )((_))\((_)()\(()\ )(_))/(/(   
(_)) _(_/( (()(_)(()((_)((_|(_)_((_)_\  
/ -_) ' \)\) _` |\ V  V / '_/ _` | '_ \) 
\___|_||_|\__, | \_/\_/|_| \__,_| .__/  
          |___/                 |_|     
------------------------------------------------
Usage: engwrap <command> [flags]

A simple docker wrapper to manage red team environments.

Flags:
  -h, --help     Show context-sensitive help.
      --debug    Enable debug mode.

Commands:
  create           Create and start a new environment.
  enter            Spawn a shell in an existing environment.
  stop             Stop a running environment.
  destroy          Permanently remove an environment.
  archive          Export workspace to file.
  list             List all configured environments.
  template list    List available templates
  template add     Add a new template
```

<details>
<summary>Cheatsheet</summary>

Create a new environment from a template, rename it and start it in interactive mode:
```bash
engwrap create -t kali-rolling -n redteam -i
```

Enter the environment:
```bash
engwrap enter redteam
```

Stop the environment:
```bash
engwrap stop redteam
```

Destroy the environment:
```bash
engwrap destroy redteam
```

Archive the environment:
```bash
engwrap archive redteam redteam.tar.gz
```

List all environments:
```bash
engwrap list
```

List available templates:
```bash
engwrap template list
```

Add a new template:
```bash
engwrap template add /path/to/template.yaml
```
</details>

## templating

Each environment is defined by a simple YAML file:

```yaml
name: redteam
image: kalilinux/kali-rolling:latest
mounts:
  - /home/user/wordlists:/wordlists:ro
networks:
  - host
init_commands:
  - apt-get update && apt-get install -y nmap proxychains-ng
shell_prompt:
  - export PS1="($(date +%d/%m/%Y\ -\ %H:%M)) ${ENV_NAME} \w> "
```

## building

```bash
make
```

## logging

Every environment keeps its own complete log trail under `$HOME/.engwrap/work/<env_name>`:

* `cmdlog.log` — every executed command with timestamp
* `session-<timestamp>.typescript` — full terminal recording
* `docker.log` — container engine output

If something breaks, you'll know why. And who did it.

<small>(Yes, it was you. It's always you.)</small>

## disclaimer
Could this be a bash script packed with shenanigans? Absolutely.

Why go with this approach? Simple: I'm far more comfortable doing it this way than wrestling with yet another script. Parsing a YAML file becomes trivial, and adding new features later won't feel like performing dark magic.

I did use some kind of a script before ([distrobox](https://github.com/89luca89/distrobox/)) but it never quite lived up to my expectations. So here we are, at the end of the day, this is just another vibecoded creation wandering through the internet. It solves my headache, and with a bit of luck it might ease yours too.

*Don't overthink it: it works for my setup and my habits. If it works with yours, great. If not... the world is full of containers and questionable life choices.*