const steps = [
  {
    n: '01',
    title: 'Add what you follow',
    body: 'Paste a URL or drop in an OPML export. Feedoku finds the feed and pulls in the latest articles right away.',
  },
  {
    n: '02',
    title: 'We keep it fresh',
    body: 'Feeds are checked in the background with conditional requests, so you get new posts without hammering anyone’s server.',
  },
  {
    n: '03',
    title: 'Read, search, get briefed',
    body: 'Work through your unread list at keyboard speed. Summaries, briefings and Q&A are on the way.',
  },
];

export function HowItWorks() {
  return (
    <section id='how' className='scroll-mt-24 px-4 py-24'>
      <div className='mx-auto max-w-5xl rounded-[2rem] bg-primary p-8 text-primary-foreground sm:p-14'>
        <p className='text-sm font-semibold opacity-70'>How it works</p>
        <h2 className='mt-2 max-w-xl text-4xl font-extrabold tracking-tighter text-balance sm:text-5xl'>
          From scattered tabs to one calm inbox.
        </h2>
        <ol className='mt-12 grid gap-8 md:grid-cols-3'>
          {steps.map((s) => (
            <li key={s.n}>
              <span className='text-5xl font-extrabold tracking-tighter opacity-30'>
                {s.n}
              </span>
              <h3 className='mt-3 text-xl font-bold tracking-tight'>
                {s.title}
              </h3>
              <p className='mt-2 leading-relaxed opacity-80'>{s.body}</p>
            </li>
          ))}
        </ol>
      </div>
    </section>
  );
}
