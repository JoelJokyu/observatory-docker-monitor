export interface Sample {
  container_id: string;
  at: string;
  cpu_percent: number | null;
  memory_bytes: number;
  memory_limit_bytes: number;
  network_rx_bytes: number | null;
  network_tx_bytes: number | null;
  block_read_bytes: number | null;
  block_write_bytes: number | null;
  pids: number | null;
  network_rx_bytes_per_second: number | null;
  network_tx_bytes_per_second: number | null;
  block_read_bytes_per_second: number | null;
  block_write_bytes_per_second: number | null;
}
export interface Container {
  recorded_only?: boolean;
  project: string;
  service: string;
  id: string;
  name: string;
  image: string;
  state: string;
  present: boolean;
  seen_at: string;
  latest: Sample | null;
}
export interface Collector {
  last_cycle: string | null;
  last_inventory: string | null;
  last_sample: string | null;
  healthy: boolean;
  errors: number;
  message: string;
}
export interface Inventory {
  projects?: string[];
  containers: Container[];
  collector: Collector;
  server_time: string;
  sample_interval_seconds: number;
}
export interface Bucket {
  at: string;
  first: string;
  last: string;
  count: number;
  cpu_percent: number | null;
  memory_bytes: number;
  memory_limit_bytes: number;
  pids: number | null;
  network_rx_bytes_per_second: number | null;
  network_tx_bytes_per_second: number | null;
  block_read_bytes_per_second: number | null;
  block_write_bytes_per_second: number | null;
}
export interface History {
  from: string;
  to: string;
  bucket_seconds: number;
  aggregation: string;
  samples: Bucket[];
}
export type Metric =
  | 'cpu_percent'
  | 'memory_bytes'
  | 'pids'
  | 'network_rx_bytes_per_second'
  | 'network_tx_bytes_per_second'
  | 'block_read_bytes_per_second'
  | 'block_write_bytes_per_second';
