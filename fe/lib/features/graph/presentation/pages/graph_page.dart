import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';

import '../../../../core/di/injection.dart';
import '../../../../core/error/result.dart';
import '../../../../core/theme/app_colors.dart';
import '../../../person/domain/usecases/get_person.dart';
import '../../../person/presentation/widgets/person_card_view.dart';
import '../../domain/entities/graph_data.dart';
import '../bloc/graph_bloc.dart';
import '../layout/force_directed_layout.dart';
import '../widgets/graph_canvas.dart';
import '../widgets/node_chip.dart';

/// Spazio del grafo: tutte le persone e i luoghi come nodi collegati dalle loro
/// relazioni. Carica /graph e li dispone su un canvas con layout force-directed;
/// pan/zoom via InteractiveViewer; tap su una persona apre la sua scheda.
class GraphPage extends StatelessWidget {
  const GraphPage({super.key, this.highlightIds = const {}, this.bloc});

  /// Id dei nodi da evidenziare all'apertura (es. le persone toccate dall'ultimo
  /// messaggio, o il nodo da cui si è aperto "vedi nel grafo").
  final Set<int> highlightIds;

  /// Bloc iniettabile per i test (con un grafo finto). In produzione è null e la
  /// pagina ne crea uno via DI che carica /graph.
  final GraphBloc? bloc;

  @override
  Widget build(BuildContext context) {
    return BlocProvider(
      create: (_) =>
          bloc ?? (GraphBloc(getGraph: sl())..add(const GraphLoadRequested())),
      child: _GraphView(highlightIds: highlightIds),
    );
  }
}

class _GraphView extends StatelessWidget {
  const _GraphView({required this.highlightIds});
  final Set<int> highlightIds;

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: AppColors.background,
      appBar: AppBar(
        backgroundColor: AppColors.background,
        elevation: 0,
        title: const Text('Spazio', style: TextStyle(color: AppColors.textPrimary)),
        iconTheme: const IconThemeData(color: AppColors.textPrimary),
      ),
      body: BlocBuilder<GraphBloc, GraphState>(
        builder: (context, state) {
          switch (state.status) {
            case GraphStatus.initial:
            case GraphStatus.loading:
              return const Center(child: CircularProgressIndicator());
            case GraphStatus.error:
              return Center(
                child: Text(
                  state.errorMessage ?? 'Errore nel caricamento',
                  style: const TextStyle(color: AppColors.textSecondary),
                ),
              );
            case GraphStatus.loaded:
              if (state.graph.nodes.isEmpty) {
                return const Center(
                  child: Text(
                    'Ancora nessuna relazione da mostrare',
                    style: TextStyle(color: AppColors.textSecondary),
                  ),
                );
              }
              return _GraphCanvas(
                graph: state.graph,
                highlightIds: highlightIds,
              );
          }
        },
      ),
    );
  }
}

class _GraphCanvas extends StatelessWidget {
  const _GraphCanvas({required this.graph, this.highlightIds = const {}});
  final GraphData graph;
  final Set<int> highlightIds;

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(
      builder: (context, constraints) {
        // Il canvas riempie ESATTAMENTE il viewport: così all'apertura tutti i
        // nodi sono visibili senza bisogno di zoom-to-fit. Il layout li dispone
        // dentro questi confini (con padding interno per non sbordare). L'utente
        // può comunque fare pinch-zoom e pan per avvicinarsi (constrained:true,
        // niente canvas gigante che lasciava la vista iniziale su un angolo vuoto).
        final size = Size(constraints.maxWidth, constraints.maxHeight);
        final positions = forceDirectedLayout(graph, size);

        return InteractiveViewer(
          minScale: 0.5,
          maxScale: 4.0,
          boundaryMargin: const EdgeInsets.all(80),
          child: SizedBox(
            width: size.width,
            height: size.height,
            child: Stack(
              children: [
                // Archi (linee) sotto i nodi. CustomPaint con size ESPLICITA: sotto
                // i vincoli illimitati di InteractiveViewer(constrained:false) un
                // Positioned.fill non dimensiona lo Stack e l'intera vista collassa
                // a zero (schermata vuota). La size esplicita evita il collasso.
                CustomPaint(
                  size: size,
                  painter: GraphEdgesPainter(
                    edges: graph.edges,
                    positions: positions,
                  ),
                ),
                // Nodi: chip centrati sulla loro posizione. Tap su una PERSONA
                // apre la sua scheda; i luoghi non hanno scheda → no-op.
                for (final node in graph.nodes)
                  if (positions[node.id] case final p?)
                    Positioned(
                      left: p.dx,
                      top: p.dy,
                      child: FractionalTranslation(
                        translation: const Offset(-0.5, -0.5),
                        child: NodeChip(
                          node: node,
                          highlighted: highlightIds.contains(node.id),
                          onTap: node.isPerson
                              ? () => _openPersonCard(context, node.id)
                              : () {},
                        ),
                      ),
                    ),
              ],
            ),
          ),
        );
      },
    );
  }
}

/// Apre la scheda della persona [id] in un bottom sheet, caricandola via il
/// usecase GetPerson esistente (riuso di PersonCardView, nessuna nuova UI).
void _openPersonCard(BuildContext context, int id) {
  showModalBottomSheet<void>(
    context: context,
    backgroundColor: AppColors.background,
    isScrollControlled: true,
    showDragHandle: true,
    builder: (_) => _PersonCardSheet(personId: id),
  );
}

class _PersonCardSheet extends StatelessWidget {
  const _PersonCardSheet({required this.personId});
  final int personId;

  @override
  Widget build(BuildContext context) {
    return FractionallySizedBox(
      heightFactor: 0.85,
      child: FutureBuilder<Result>(
        future: sl<GetPerson>().call(personId),
        builder: (context, snapshot) {
          if (!snapshot.hasData) {
            return const Center(child: CircularProgressIndicator());
          }
          final result = snapshot.data!;
          return switch (result) {
            Success(:final data) => PersonCardView(person: data),
            Error(:final failure) => Center(
                child: Text(
                  failure.message,
                  style: const TextStyle(color: AppColors.textSecondary),
                ),
              ),
          };
        },
      ),
    );
  }
}

