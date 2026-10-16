import { ChevronDownIcon } from 'lucide-react';

const faqs = [
  {
    q: 'Is Feedoku free?',
    a: 'Feedoku is open source. You can self-host it with a single docker compose file. A hosted version is not available yet.',
  },
  {
    q: 'Can I bring my feeds from another reader?',
    a: 'Yes. Export an OPML file from your current reader and import it from the Add feed dialog. Folders are preserved.',
  },
  {
    q: 'Which feed formats work?',
    a: 'RSS, Atom and JSON Feed. If you only have a website address, Feedoku looks for the feed automatically.',
  },
  {
    q: 'How will the AI features work?',
    a: 'Each article is summarized once and shared across everyone who follows that feed. Summaries and Q&A will use open models, and answers always link back to the articles they came from. These features are in development.',
  },
  {
    q: 'Does Feedoku republish other people’s content?',
    a: 'No. Articles are fetched from the original feed for you to read, and every article links back to its source.',
  },
];

export function Faq() {
  return (
    <section id='faq' className='scroll-mt-24 px-4 py-24'>
      <div className='mx-auto max-w-3xl'>
        <p className='text-sm font-semibold text-brand'>FAQ</p>
        <h2 className='mt-2 text-4xl font-extrabold tracking-tighter text-balance sm:text-5xl'>
          Questions, answered.
        </h2>
        <div className='mt-10 space-y-3'>
          {faqs.map(({ q, a }) => (
            <details
              key={q}
              className='group rounded-3xl border bg-card/70 px-6 py-5 backdrop-blur open:shadow-[var(--shadow-panel)]'
            >
              <summary className='flex cursor-pointer list-none items-center justify-between gap-4 font-semibold [&::-webkit-details-marker]:hidden'>
                {q}
                <ChevronDownIcon className='size-5 shrink-0 text-muted-foreground transition-transform group-open:rotate-180' />
              </summary>
              <p className='mt-3 leading-relaxed text-muted-foreground'>{a}</p>
            </details>
          ))}
        </div>
      </div>
    </section>
  );
}
