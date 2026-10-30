ALTER TABLE samples
 ADD COLUMN network_rx_bytes bigint CHECK (network_rx_bytes >= 0),
 ADD COLUMN network_tx_bytes bigint CHECK (network_tx_bytes >= 0),
 ADD COLUMN block_read_bytes bigint CHECK (block_read_bytes >= 0),
 ADD COLUMN block_write_bytes bigint CHECK (block_write_bytes >= 0),
 ADD COLUMN pids bigint CHECK (pids >= 0),
 ADD COLUMN network_rx_bytes_per_second double precision CHECK (network_rx_bytes_per_second >= 0),
 ADD COLUMN network_tx_bytes_per_second double precision CHECK (network_tx_bytes_per_second >= 0),
 ADD COLUMN block_read_bytes_per_second double precision CHECK (block_read_bytes_per_second >= 0),
 ADD COLUMN block_write_bytes_per_second double precision CHECK (block_write_bytes_per_second >= 0);