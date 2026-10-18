import { useCallback, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { errorMessage } from './documentsApi';

/** useWrite runs one write at a time for a page of the Documenti view: busy while it runs, and
 * the server's sentence under failedKey when it fails. */
export function useWrite(failedKey: string) {
  const { t } = useTranslation();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const run = useCallback(
    (work: () => Promise<void>) => {
      setBusy(true);
      setError('');
      work()
        .catch((err: unknown) => {
          setError(t(failedKey, { message: errorMessage(err) }));
        })
        .finally(() => {
          setBusy(false);
        });
    },
    [t, failedKey],
  );
  return { busy, error, run };
}
