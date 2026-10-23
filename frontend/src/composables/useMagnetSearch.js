import { ref, onUnmounted } from 'vue';
import { StreamEpisodeMagnets } from '../api.js';

export function useMagnetSearch() {
  const searching = ref(false);
  const candidates = ref([]);
  const error = ref('');
  let active;

  const cancel = () => {
    active?.abort();
    active = undefined;
    searching.value = false;
  };
  const search = async (id, episode, keywords) => {
    cancel();
    const controller = new AbortController();
    active = controller;
    candidates.value = [];
    error.value = '';
    searching.value = true;
    try {
      await StreamEpisodeMagnets(id, episode, keywords, (items) => {
        if (active === controller) candidates.value = items;
      }, controller.signal);
    } catch (err) {
      if (active === controller && !controller.signal.aborted) error.value = String(err);
    } finally {
      if (active === controller) {
        searching.value = false;
        active = undefined;
      }
    }
  };
  onUnmounted(cancel);
  return { searching, candidates, error, search, cancel };
}
