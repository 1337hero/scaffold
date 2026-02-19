export const deskData = {
  theOne: {
    id: '1',
    title: 'Set up Preact frontend',
    project: { name: 'Scaffold', color: 'amber' },
    done: false,
    microSteps: [
      { id: 's1', text: 'Scaffold Vite + Preact project', done: true },
      { id: 's2', text: 'Wire Tailwind 4 theme', done: false },
      { id: 's3', text: 'Build Desk component', done: false },
      { id: 's4', text: 'Connect /inbox API', done: false },
    ],
  },
  tasks: [
    {
      id: '2',
      num: 2,
      title: 'Invoice TechGuys for January',
      project: { name: '1337hero', color: 'blue' },
      done: false,
    },
    {
      id: '3',
      num: 3,
      title: 'Write for 30 min \u2014 Chapter 3',
      project: { name: 'Book', color: 'purple' },
      done: false,
    },
  ],
  doneToday: [
    { id: 'd1', title: 'Morning workout \u2014 45min strength' },
  ],
}

export const inboxData = {
  groups: [
    {
      id: 'do',
      label: 'Do',
      color: 'amber',
      items: [
        {
          id: 'i1',
          type: 'task',
          title: 'Renew domain for mk3y.com \u2014 expires March 2',
          summary: '12 days out. Auto-classified as deadline task.',
          time: '11:15a',
          actions: ['Add to desk', 'Snooze', 'Archive'],
        },
        {
          id: 'i2',
          type: 'task',
          title: 'Add security headers to 1337hero Caddyfile',
          summary: 'CSP, HSTS, X-Frame-Options.',
          time: 'Mon 9p',
          actions: ['Add to desk', 'Snooze', 'Archive'],
        },
      ],
    },
    {
      id: 'explore',
      label: 'Explore',
      color: 'purple',
      items: [
        {
          id: 'i3',
          type: 'video',
          title: 'I Gave Claude Access to My Entire Knowledge Base',
          summary: 'Brad Morris \u2014 RA-H OS. SQLite knowledge graph, MCP bridge, hub nodes.',
          time: '2:15p',
          actions: ['Open notebook', 'Extract tasks', 'Archive'],
        },
        {
          id: 'i4',
          type: 'idea',
          title: 'What if Knitly had a yarn marketplace',
          summary: 'Transaction fee model. Connect to pattern system.',
          time: '10:32a',
          actions: ['Open notebook', 'Make task', 'Archive'],
        },
        {
          id: 'i5',
          type: 'link',
          title: 'Bun v1.2 \u2014 SQLite improvements',
          summary: 'WAL mode, prepared statement caching. Relevant to Scaffold.',
          time: '9:45a',
          actions: ['Open notebook', 'Archive'],
        },
      ],
    },
    {
      id: 'reference',
      label: 'Reference',
      color: 'cyan',
      items: [
        {
          id: 'i6',
          type: 'article',
          title: 'ADHD and External Scaffolding: What the Research Says',
          summary: 'Externalization, body doubling, implementation intentions.',
          time: 'Mon 8p',
          actions: [],
        },
        {
          id: 'i7',
          type: 'note',
          title: 'The waiting mode problem',
          summary: 'Meeting at 11 kills 9\u201311. Anticipatory paralysis.',
          time: 'Mon 2p',
          actions: [],
        },
      ],
    },
  ],
}

export const notebooksData = [
  {
    id: 'nb1',
    title: 'ADHD System Design',
    desc: 'Zen Desk concept, research, architecture convergence',
    icon: '\u26a1',
    iconBg: 'amber',
    nodes: 12,
    updated: 'Updated today',
  },
  {
    id: 'nb2',
    title: 'Knitly',
    desc: 'Product decisions, features, architecture',
    icon: '\ud83e\uddf6',
    iconBg: 'purple',
    nodes: 23,
    updated: 'Updated yesterday',
  },
  {
    id: 'nb3',
    title: 'Shenandoah',
    desc: 'Story bible, March, Genesis, the 77 worlds',
    icon: '\u270d\ufe0f',
    iconBg: 'green',
    nodes: 41,
    updated: 'Updated 3 days ago',
  },
  {
    id: 'nb4',
    title: 'Homelab',
    desc: 'Infrastructure, services, network',
    icon: '\ud83c\udfe0',
    iconBg: 'blue',
    nodes: 15,
    updated: 'Updated Feb 16',
  },
  {
    id: 'nb5',
    title: '1337hero',
    desc: 'Business, clients, onboarding',
    icon: '\ud83d\udcbc',
    iconBg: 'red',
    nodes: 9,
    updated: 'Updated Feb 14',
  },
]

export const notebookThread = {
  id: 'nb1',
  title: 'ADHD System Design',
  messages: [
    {
      id: 'm1',
      author: 'you',
      text: 'Bring me up to speed on the RA-H video I saved',
    },
    {
      id: 'm2',
      author: 'system',
      html: `Same graph pattern \u2014 nodes + typed edges + SQLite. MCP bridge so any agent reads/writes it. Hub nodes surface by connection count, not manual ranking.
<br/><br/><strong>Key difference:</strong> RA-H is knowledge-first. Yours is execution-first (the Desk) with knowledge underneath.`,
    },
    {
      id: 'm3',
      author: 'you',
      text: 'What should I steal from it?',
    },
    {
      id: 'm4',
      author: 'system',
      html: `Three things:
<ul>
<li><strong>URL ingest pipeline</strong> \u2014 drop a link, it auto-fetches + summarizes + embeds</li>
<li><strong>Guides-as-skills</strong> \u2014 markdown docs teach the agent how to organize</li>
<li><strong>Context view</strong> \u2014 most-connected nodes first \u2014 this IS the cortex bulletin pattern</li>
</ul>`,
    },
  ],
  relatedNodes: [
    { id: 'rn1', title: 'Scaffold Architecture Convergence Map', type: 'note', date: 'Created Feb 18' },
    { id: 'rn2', title: 'vera-cortex Implementation Spec', type: 'ref', date: 'Created Feb 17' },
    { id: 'rn3', title: 'ADHD External Scaffolding Research', type: 'ref', date: 'Captured Mon' },
  ],
}

export const inboxCount = inboxData.groups.reduce((sum, g) => sum + g.items.length, 0)

export const projectColors = {
  amber: { text: 'text-amber', bg: 'bg-amber-dim', dot: 'bg-amber' },
  blue: { text: 'text-blue', bg: 'bg-blue-dim', dot: 'bg-blue' },
  purple: { text: 'text-purple', bg: 'bg-purple-dim', dot: 'bg-purple' },
  green: { text: 'text-green', bg: 'bg-green-dim', dot: 'bg-green' },
  cyan: { text: 'text-cyan', bg: 'bg-cyan/10', dot: 'bg-cyan' },
  red: { text: 'text-red', bg: 'bg-red-dim', dot: 'bg-red' },
}

export const colorClass = (color, variant = 'text') =>
  projectColors[color]?.[variant] ?? 'text-text-dim'

export const typeStyles = {
  task: 'bg-amber-dim text-amber',
  link: 'bg-blue-dim text-blue',
  idea: 'bg-purple-dim text-purple',
  video: 'bg-red-dim text-red',
  note: 'bg-green-dim text-green',
  article: 'bg-[rgba(6,182,212,0.1)] text-cyan',
  ref: 'bg-[rgba(6,182,212,0.1)] text-cyan',
}

export function getDeskData() { return { theOne: deskData.theOne, tasks: deskData.tasks } }
export function getInboxData() { return inboxData.groups }
export function getNotebooks() { return notebooksData }
export function getNotebook(id) { return notebookThread }
