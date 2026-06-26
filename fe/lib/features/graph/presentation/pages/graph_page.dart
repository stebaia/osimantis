import 'dart:math' as math;

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

class _GraphCanvas extends StatefulWidget {
  const _GraphCanvas({required this.graph, this.highlightIds = const {}});
  final GraphData graph;
  final Set<int> highlightIds;

  @override
  State<_GraphCanvas> createState() => _GraphCanvasState();
}

class _GraphCanvasState extends State<_GraphCanvas> {
  final _controller = TransformationController();
  bool _fitted = false;

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  /// Imposta UNA volta la trasformazione iniziale così l'intero canvas entra nel
  /// viewport, centrato (zoom-to-fit). Dopo, l'utente è libero di pan/zoom.
  void _fitToViewport(Size viewport, Size canvas) {
    if (_fitted) return;
    _fitted = true;
    // Scala per far stare il canvas nel viewport, con un piccolo margine.
    final scale = math.min(
          viewport.width / canvas.width,
          viewport.height / canvas.height,
        ) *
        0.92;
    // Centra: trasla così il centro del canvas cada al centro del viewport.
    final dx = (viewport.width - canvas.width * scale) / 2;
    final dy = (viewport.height - canvas.height * scale) / 2;
    _controller.value = Matrix4.identity()
      ..translateByDouble(dx, dy, 0, 1)
      ..scaleByDouble(scale, scale, 1, 1);
  }

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(
      builder: (context, constraints) {
        final viewport = Size(constraints.maxWidth, constraints.maxHeight);

        // Spazio di disegno GRANDE e quadrato: i nodi vivono al centro con aria
        // attorno, e l'intero spazio è liberamente navigabile (pan/zoom) senza
        // bordi che taglino i chip. ~320px di "stanza" per nodo, con un minimo.
        final side = math.max(
          math.max(viewport.width, viewport.height),
          widget.graph.nodes.length * 320.0,
        );
        final canvas = Size(side, side);
        final positions = forceDirectedLayout(widget.graph, canvas);

        // Vista iniziale: scala e centra lo spazio così tutti i nodi sono visibili
        // all'apertura (zoom-to-fit). Senza, partiremmo su un angolo vuoto.
        WidgetsBinding.instance.addPostFrameCallback((_) {
          _fitToViewport(viewport, canvas);
        });

        return InteractiveViewer(
          transformationController: _controller,
          minScale: 0.1,
          maxScale: 4.0,
          // boundaryMargin enorme = spazio "infinito" navigabile in ogni direzione.
          boundaryMargin: const EdgeInsets.all(double.infinity),
          constrained: false,
          clipBehavior: Clip.none,
          child: SizedBox(
            width: canvas.width,
            height: canvas.height,
            child: Stack(
              clipBehavior: Clip.none,
              children: [
                CustomPaint(
                  size: canvas,
                  painter: GraphEdgesPainter(
                    edges: widget.graph.edges,
                    positions: positions,
                  ),
                ),
                // Nodi: chip centrati sulla loro posizione. Tap su una PERSONA
                // apre la sua scheda; i luoghi non hanno scheda → no-op.
                for (final node in widget.graph.nodes)
                  if (positions[node.id] case final p?)
                    Positioned(
                      left: p.dx,
                      top: p.dy,
                      child: FractionalTranslation(
                        translation: const Offset(-0.5, -0.5),
                        child: NodeChip(
                          node: node,
                          highlighted: widget.highlightIds.contains(node.id),
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

