'use client';

import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { api } from '@/lib/reader-api';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { PlusIcon } from 'lucide-react';
import { useRef, useState } from 'react';
import { toast } from 'sonner';

export function AddFeedDialog() {
  const [open, setOpen] = useState(false);
  const [url, setUrl] = useState('');
  const fileRef = useRef<HTMLInputElement>(null);
  const qc = useQueryClient();

  const refresh = () => {
    qc.invalidateQueries({ queryKey: ['feeds'] });
    qc.invalidateQueries({ queryKey: ['articles'] });
  };

  const add = useMutation({
    mutationFn: api.addFeed,
    onSuccess: (feed) => {
      toast.success(`Subscribed to ${feed.title || feed.url}`);
      setUrl('');
      setOpen(false);
      refresh();
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const importOpml = useMutation({
    mutationFn: async (file: File) => api.importOpml(await file.text()),
    onSuccess: ({ imported, failed }) => {
      toast.success(
        `Imported ${imported} feed${imported === 1 ? '' : 's'}` +
          (failed.length ? `, ${failed.length} couldn't be reached` : '')
      );
      setOpen(false);
      refresh();
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const busy = add.isPending || importOpml.isPending;

  return (
    <>
      <Button
        variant='ghost'
        size='icon'
        aria-label='Add feed'
        className='size-7 text-muted-foreground'
        onClick={() => setOpen(true)}
      >
        <PlusIcon className='size-4' />
      </Button>
      <Dialog open={open} onOpenChange={(o) => !busy && setOpen(o)}>
        <DialogContent>
          <DialogTitle>Add a feed</DialogTitle>
          <DialogDescription>
            Paste a website or feed address. We&apos;ll find the feed for you.
          </DialogDescription>
          <form
            className='grid gap-3'
            onSubmit={(e) => {
              e.preventDefault();
              if (url.trim()) add.mutate(url.trim());
            }}
          >
            <Input
              required
              placeholder='https://example.com'
              inputMode='url'
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              disabled={busy}
              aria-label='Feed or website URL'
            />
            <Button type='submit' disabled={busy || !url.trim()}>
              {add.isPending ? 'Looking for feed…' : 'Subscribe'}
            </Button>
          </form>
          <div className='border-t pt-4'>
            <p className='mb-2 text-sm text-muted-foreground'>
              Coming from another reader?
            </p>
            <input
              ref={fileRef}
              type='file'
              accept='.opml,.xml,text/xml,text/x-opml'
              hidden
              onChange={(e) => {
                const file = e.target.files?.[0];
                if (file) importOpml.mutate(file);
                e.target.value = '';
              }}
            />
            <Button
              variant='outline'
              disabled={busy}
              onClick={() => fileRef.current?.click()}
            >
              {importOpml.isPending ? 'Importing…' : 'Import OPML file'}
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    </>
  );
}
