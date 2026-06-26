part of 'graph_bloc.dart';

sealed class GraphEvent extends Equatable {
  const GraphEvent();

  @override
  List<Object?> get props => [];
}

/// Richiede il caricamento (o ricaricamento) dell'intero grafo.
class GraphLoadRequested extends GraphEvent {
  const GraphLoadRequested();
}
