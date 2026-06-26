import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:osimantis/features/graph/domain/entities/graph_data.dart';
import 'package:osimantis/features/graph/presentation/widgets/node_chip.dart';

Widget _wrap(Widget child) => MaterialApp(home: Scaffold(body: Center(child: child)));

void main() {
  testWidgets('NodeChip mostra il nome e risponde al tap', (tester) async {
    var tapped = false;
    await tester.pumpWidget(_wrap(NodeChip(
      node: const GraphNode(id: 1, type: 'person', name: 'Mura'),
      onTap: () => tapped = true,
    )));

    expect(find.text('Mura'), findsOneWidget);
    expect(find.byIcon(Icons.person), findsOneWidget);

    await tester.tap(find.byType(NodeChip));
    expect(tapped, isTrue);
  });

  testWidgets('un luogo usa l\'icona place', (tester) async {
    await tester.pumpWidget(_wrap(NodeChip(
      node: const GraphNode(id: 8, type: 'place', name: 'Rift'),
      onTap: () {},
    )));
    expect(find.byIcon(Icons.place), findsOneWidget);
  });

  testWidgets('il nodo utente usa l\'icona star', (tester) async {
    await tester.pumpWidget(_wrap(NodeChip(
      node: const GraphNode(id: 7, type: 'person', name: 'Io', data: {'is_user': true}),
      onTap: () {},
    )));
    expect(find.byIcon(Icons.star), findsOneWidget);
  });
}
